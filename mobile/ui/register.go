package ui

import (
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// RegisterScreen mirrors LoginScreen but posts to /auth/register.
type RegisterScreen struct {
	app *App

	githubBtn widget.Clickable

	email    widget.Editor
	password widget.Editor

	registerBtn widget.Clickable
	backBtn     widget.Clickable

	mu      sync.Mutex
	uiReset bool // consumed by Layout (editors are UI-goroutine only)
	loading bool
	errMsg  string
}

func newRegisterScreen(app *App) *RegisterScreen {
	s := &RegisterScreen{app: app}
	s.email.SingleLine = true
	s.password.SingleLine = true
	s.password.Mask = '•'
	return s
}

func (s *RegisterScreen) setLoading(v bool) {
	s.mu.Lock()
	s.loading = v
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

func (s *RegisterScreen) setError(msg string) {
	s.mu.Lock()
	s.errMsg = msg
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

func (s *RegisterScreen) snapshot() (loading bool, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loading, s.errMsg
}

func (s *RegisterScreen) submit() {
	email := strings.TrimSpace(s.email.Text())
	password := s.password.Text()
	if email == "" || password == "" {
		s.setError("Enter an email and password.")
		return
	}
	if len(password) < 6 {
		s.setError("Password should be at least 6 characters.")
		return
	}
	s.setError("")
	s.setLoading(true)

	go func() {
		token, err := s.app.Client.Register(email, password)
		if err != nil {
			s.setLoading(false)
			s.setError(err.Error())
			return
		}
		s.setLoading(false)
		s.app.BeginSession(token)
	}()
}

// Layout draws the Register screen and processes its own button clicks.
func (s *RegisterScreen) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	for s.registerBtn.Clicked(gtx) {
		s.submit()
	}
	for s.backBtn.Clicked(gtx) {
		s.setError("")
		s.app.ShowLogin()
	}

	s.mu.Lock()
	doReset := s.uiReset
	s.uiReset = false
	s.mu.Unlock()
	if doReset {
		s.email.SetText("")
		s.password.SetText("")
	}

	loading, errMsg := s.snapshot()
	s.email.ReadOnly = loading
	s.password.ReadOnly = loading

	return authScreen(gtx, th, "Create account", "Index your first repository in under a minute.",
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(AuthField(th, &s.email, "Email", "you@example.com")),
				layout.Rigid(authGap(20)),
				layout.Rigid(AuthField(th, &s.password, "Password", "Atleast 8 characters")),
				layout.Rigid(authGap(8)),
				layout.Rigid(ErrorText(th, errMsg)),
				layout.Rigid(authGap(24)),
				layout.Rigid(AuthButton(th, &s.registerBtn, "Register", loading)),
				layout.Rigid(authGap(16)),
				layout.Rigid(AuthLink(th, &s.backBtn, "Already have an account?")),
				layout.Rigid(authGap(8)),
				layout.Rigid(AuthGitHubButton(&s.githubBtn)),
			)
		})
}

// reset drops everything typed or shown for the previous account.
func (s *RegisterScreen) reset() {
	s.mu.Lock()
	s.loading = false
	s.errMsg = ""
	s.uiReset = true
	s.mu.Unlock()
}
