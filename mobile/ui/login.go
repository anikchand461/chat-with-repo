package ui

import (
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"chatwithrepo/mobile/api"
)

// LoginScreen holds all state for the Login screen: the two text
// fields, the two buttons, and a small loading/error status.
type LoginScreen struct {
	app *App

	githubBtn widget.Clickable

	email    widget.Editor
	password widget.Editor

	loginBtn    widget.Clickable
	registerBtn widget.Clickable

	mu      sync.Mutex
	loading bool
	errMsg  string
}

func newLoginScreen(app *App) *LoginScreen {
	s := &LoginScreen{app: app}
	s.email.SingleLine = true
	s.password.SingleLine = true
	s.password.Mask = '•'
	return s
}

func (s *LoginScreen) setLoading(v bool) {
	s.mu.Lock()
	s.loading = v
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

func (s *LoginScreen) setError(msg string) {
	s.mu.Lock()
	s.errMsg = msg
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

func (s *LoginScreen) snapshot() (loading bool, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loading, s.errMsg
}

func (s *LoginScreen) submit() {
	email := strings.TrimSpace(s.email.Text())
	password := s.password.Text()
	if email == "" || password == "" {
		s.setError("Enter your email and password.")
		return
	}
	s.setError("")
	s.setLoading(true)

	go func() {
		token, err := s.app.Client.Login(email, password)
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

// Layout draws the Login screen and processes its own button clicks.
func (s *LoginScreen) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	for s.loginBtn.Clicked(gtx) {
		s.submit()
	}
	for s.registerBtn.Clicked(gtx) {
		s.setError("")
		s.app.ShowRegister()
	}

	loading, errMsg := s.snapshot()

	return authScreen(gtx, th, "Welcome back", "Log in to continue to your repository chats.",
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(AuthField(th, &s.email, "Email", "you@example.com")),
				layout.Rigid(authGap(20)),
				layout.Rigid(AuthField(th, &s.password, "Password", "Enter your password")),
				layout.Rigid(authGap(8)),
				layout.Rigid(ErrorText(th, errMsg)),
				layout.Rigid(authGap(24)),
				layout.Rigid(AuthButton(th, &s.loginBtn, "Continue", loading)),
				layout.Rigid(authGap(16)),
				layout.Rigid(AuthLink(th, &s.registerBtn, "Create an account")),
				layout.Rigid(authGap(8)),
				layout.Rigid(AuthGitHubButton(&s.githubBtn)),
			)
		})
}