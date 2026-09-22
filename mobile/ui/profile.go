package ui

import (
	"image/color"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"chatwithrepo/mobile/api"
)

// Amber/green status colors, matching the web frontend's profile.html
// (--warn-text / --ok on the "GitHub token configured / not configured"
// banner).
var (
	profileWarnBg     = color.NRGBA{R: 0x3a, G: 0x2b, B: 0x0a, A: 0xff}
	profileWarnBorder = color.NRGBA{R: 0x6b, G: 0x51, B: 0x12, A: 0xff}
	profileWarnText   = color.NRGBA{R: 0xfb, G: 0xbf, B: 0x24, A: 0xff}
	profileOkBg       = color.NRGBA{R: 0x0e, G: 0x2f, B: 0x1e, A: 0xff}
	profileOkBorder   = color.NRGBA{R: 0x1f, G: 0x5c, B: 0x3a, A: 0xff}
	profileOkText     = color.NRGBA{R: 0x4a, G: 0xde, B: 0x80, A: 0xff}
)

// ProfileScreen mirrors the web frontend's profile.html: shows who is
// logged in and lets them save their personal GitHub access token
// (POST /auth/github-token) - raising their GitHub API limit from
// 60 to 5000 requests/hour, which indexing a repo generally needs.
//
// Reachable from the Dashboard drawer ("Profile", between the greeting
// and "GitHub" - see dashboard.go's drawer()).
type ProfileScreen struct {
	app *App

	mu        sync.Mutex
	uiReset   bool // consumed by Layout (editor is UI-goroutine only)
	loading   bool // checking /auth/me on entry
	saving    bool
	email     string
	hasToken  bool
	errMsg    string
	statusMsg string

	token   widget.Editor
	saveBtn widget.Clickable
	backBtn widget.Clickable
}

func newProfileScreen(app *App) *ProfileScreen {
	s := &ProfileScreen{app: app}
	s.token.SingleLine = true
	s.token.Mask = '•'
	return s
}

// Load fetches the current token status in the background. Called by
// App.ShowProfile every time the screen is opened, so it never shows
// stale information from a previous visit.
func (s *ProfileScreen) Load() {
	gen := s.app.Generation()
	s.mu.Lock()
	s.loading = true
	s.errMsg = ""
	s.statusMsg = ""
	s.uiReset = true
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		me, err := s.app.Client.Me()
		if !s.app.IsCurrent(gen) {
			return
		}
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}

		s.mu.Lock()
		s.loading = false
		if err != nil {
			s.errMsg = err.Error()
		} else {
			s.email = me.Email
			s.hasToken = me.HasGithubToken
		}
		s.mu.Unlock()
		s.app.Window.Invalidate()
	}()
}

func (s *ProfileScreen) snapshot() (loading, saving, hasToken bool, email, errMsg, statusMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loading, s.saving, s.hasToken, s.email, s.errMsg, s.statusMsg
}

// save sends the pasted token to the backend. A bare submit with an
// empty field just re-shows the existing state (the server never
// sends the token back, so there is nothing to "clear").
func (s *ProfileScreen) save() {
	token := strings.TrimSpace(s.token.Text())
	if token == "" {
		s.mu.Lock()
		s.errMsg = "Paste a GitHub token first."
		s.statusMsg = ""
		s.mu.Unlock()
		s.app.Window.Invalidate()
		return
	}

	gen := s.app.Generation()
	s.mu.Lock()
	s.saving = true
	s.errMsg = ""
	s.statusMsg = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		hasToken, err := s.app.Client.SaveGithubToken(token)
		if !s.app.IsCurrent(gen) {
			return
		}
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}

		s.mu.Lock()
		s.saving = false
		if err != nil {
			s.errMsg = err.Error()
		} else {
			s.hasToken = hasToken
			s.statusMsg = "Token saved."
			s.uiReset = true // clear the field; the server never echoes it back
		}
		s.mu.Unlock()
		s.app.Window.Invalidate()
	}()
}

// Layout draws the Profile screen and processes its own button clicks.
func (s *ProfileScreen) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	for s.saveBtn.Clicked(gtx) {
		s.save()
	}
	for s.backBtn.Clicked(gtx) {
		s.app.ShowDashboard()
	}

	// Android Back returns to the Dashboard (Profile is only ever reached
	// from there - ShowProfile). Without this, an unhandled Back here
	// falls through to the OS default, which quits the app instead of
	// navigating back.
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameBack})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			s.app.ShowDashboard()
		}
	}

	s.mu.Lock()
	doReset := s.uiReset
	s.uiReset = false
	s.mu.Unlock()
	if doReset {
		s.token.SetText("")
	}

	loading, saving, hasToken, email, errMsg, statusMsg := s.snapshot()
	s.token.ReadOnly = saving

	title := "Profile"
	if name := nameFromEmail(email); name != "" {
		title = "Hi " + name
	}

	return authScreen(gtx, th, title, email,
		func(gtx layout.Context) layout.Dimensions {
			rows := []layout.FlexChild{
				layout.Rigid(tokenBanner(th, loading, hasToken)),
				layout.Rigid(authGap(18)),
				layout.Rigid(AuthField(th, &s.token, "GitHub access token", "ghp_xxxxxxxxxxxxxxxxxxxx")),
				layout.Rigid(authGap(6)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, authSp(12.5), "Create a token with repo scope at github.com/settings/tokens")
					l.Color = authHint
					return l.Layout(gtx)
				}),
				layout.Rigid(authGap(10)),
				layout.Rigid(ErrorText(th, errMsg)),
			}
			if statusMsg != "" {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, statusMsg)
					l.Color = profileOkText
					return l.Layout(gtx)
				}))
			}
			rows = append(rows,
				layout.Rigid(authGap(16)),
				layout.Rigid(AuthButton(th, &s.saveBtn, "Save token", saving)),
				layout.Rigid(authGap(12)),
				layout.Rigid(AuthSecondaryButton(th, &s.backBtn, "Back to dashboard")),
			)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
}

// tokenBanner mirrors the web profile page's status banner: amber
// "not configured" or green "configured", swapped once /auth/me answers.
func tokenBanner(th *material.Theme, loading, hasToken bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		title, body := "Checking your GitHub token…", ""
		bg, border, col := authField, authFieldBorder, authHint

		switch {
		case loading:
			// keep the checking state above
		case hasToken:
			title = "GitHub token configured"
			body = "Private repos and higher rate limits are available."
			bg, border, col = profileOkBg, profileOkBorder, profileOkText
		default:
			title = "GitHub token not configured"
			body = "Without one, GitHub's public API allows only 60 requests/hour - not enough to index most repositories."
			bg, border, col = profileWarnBg, profileWarnBorder, profileWarnText
		}

		return warningBox(gtx, th, bg, border, col, title, body)
	}
}

// warningBox draws a rounded, bordered status card: a bold title line
// plus an optional muted body line below it.
func warningBox(gtx layout.Context, th *material.Theme, bg, border, titleCol color.NRGBA, title, body string) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			borderedRRect(gtx, gtx.Constraints.Min, authDp(14), bg, border)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(authDp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, authSp(14), title)
						l.Color = titleCol
						l.Font.Weight = font.Bold
						return l.Layout(gtx)
					}),
				}
				if body != "" {
					children = append(children,
						layout.Rigid(authGap(4)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(12.5), body)
							l.Color = authBody
							return l.Layout(gtx)
						}),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}),
	)
}

// reset drops everything shown for the previous account.
func (s *ProfileScreen) reset() {
	s.mu.Lock()
	s.loading = false
	s.saving = false
	s.email = ""
	s.hasToken = false
	s.errMsg = ""
	s.statusMsg = ""
	s.uiReset = true
	s.mu.Unlock()
}
