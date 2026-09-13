package ui

import (
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"chatwithrepo/mobile/api"
)

// RegisterScreen mirrors LoginScreen but posts to /auth/register.
type RegisterScreen struct {
	app *App

	email    widget.Editor
	password widget.Editor

	registerBtn widget.Clickable
	backBtn     widget.Clickable

	mu      sync.Mutex
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
		s.app.Client.SetToken(token)
		_ = api.SaveToken(token)
		s.setLoading(false)
		s.app.ShowDashboard()
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

	loading, errMsg := s.snapshot()

	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := material.H5(th, "Create Account")
					title.Color = colorText
					return title.Layout(gtx)
				}),
				layout.Rigid(spacer(6)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					sub := material.Body2(th, "Takes about ten seconds.")
					sub.Color = colorSubtleText
					return sub.Layout(gtx)
				}),
				layout.Rigid(spacer(24)),
				layout.Rigid(TextField(th, &s.email, "Email")),
				layout.Rigid(spacer(12)),
				layout.Rigid(TextField(th, &s.password, "Password")),
				layout.Rigid(spacer(8)),
				layout.Rigid(ErrorText(th, errMsg)),
				layout.Rigid(spacer(16)),
				layout.Rigid(PrimaryButton(th, &s.registerBtn, "Register", loading)),
				layout.Rigid(spacer(8)),
				layout.Rigid(TextButton(th, &s.backBtn, "Back")),
			)
		})
	})
}
