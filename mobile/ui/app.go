// Package ui contains every screen of the ChatWithRepo Android client
// plus the small App shell that switches between them. There is no
// heavyweight framework here: App just holds "which screen is active"
// and delegates Layout to that screen every frame.
package ui

import (
	"sync"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/widget/material"

	"chatwithrepo/mobile/api"
)

// Screen identifies which top-level screen is currently shown.
type Screen int

const (
	ScreenLogin Screen = iota
	ScreenRegister
	ScreenDashboard
	ScreenChat
)

// App is the single shared shell for the whole UI. It owns the API
// client, the active screen, and one state struct per screen. Screens
// call back into App (e.g. app.ShowDashboard()) to navigate.
type App struct {
	Window *app.Window
	Theme  *material.Theme
	Client *api.Client

	mu     sync.Mutex
	screen Screen

	LoginScreen    *LoginScreen
	RegisterScreen *RegisterScreen
	Dashboard      *DashboardScreen
	Chat           *ChatScreen
}

// NewApp wires up an App ready to run, starting on the Login screen.
func NewApp(w *app.Window, baseURL string) *App {
	a := &App{
		Window: w,
		Theme:  NewTheme(),
		Client: api.NewClient(baseURL),
		screen: ScreenLogin,
	}
	a.LoginScreen = newLoginScreen(a)
	a.RegisterScreen = newRegisterScreen(a)
	a.Dashboard = newDashboardScreen(a)
	a.Chat = newChatScreen(a)
	return a
}

// Bootstrap tries to resume a saved session before the first frame
// draws. If a token is stored and still valid, it jumps straight to
// the dashboard; otherwise it clears any stale token and stays on
// Login.
func (a *App) Bootstrap() {
	token, err := api.LoadToken()
	if err != nil || token == "" {
		return
	}
	a.Client.SetToken(token)
	go func() {
		if _, err := a.Client.Me(); err != nil {
			_ = api.ClearToken()
			a.Client.SetToken("")
			return
		}
		a.ShowDashboard()
	}()
}

func (a *App) currentScreen() Screen {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.screen
}

func (a *App) setScreen(s Screen) {
	a.mu.Lock()
	a.screen = s
	a.mu.Unlock()
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// ShowLogin switches to the Login screen.
func (a *App) ShowLogin() {
	a.setScreen(ScreenLogin)
}

// ShowRegister switches to the Register screen.
func (a *App) ShowRegister() {
	a.setScreen(ScreenRegister)
}

// ShowDashboard switches to the Dashboard and (re)loads the chat list.
func (a *App) ShowDashboard() {
	a.setScreen(ScreenDashboard)
	a.Dashboard.Reload()
}

// ShowChat switches to the Chat screen for the given chat and loads
// its message history.
func (a *App) ShowChat(chatID, title, branch string) {
	a.Chat.Open(chatID, title, branch)
	a.setScreen(ScreenChat)
}

// HandleUnauthorized is called whenever an API call comes back with a
// 401. It clears the session and sends the user back to Login.
func (a *App) HandleUnauthorized() {
	_ = api.ClearToken()
	a.Client.SetToken("")
	a.ShowLogin()
}

// Logout clears the session explicitly (user tapped Logout).
func (a *App) Logout() {
	_ = api.ClearToken()
	a.Client.SetToken("")
	a.ShowLogin()
}

// Layout draws whichever screen is currently active. This is called
// once per frame from main.go.
func (a *App) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return fill(gtx, colorBackground)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			switch a.currentScreen() {
			case ScreenRegister:
				return a.RegisterScreen.Layout(gtx, a.Theme)
			case ScreenDashboard:
				return a.Dashboard.Layout(gtx, a.Theme)
			case ScreenChat:
				return a.Chat.Layout(gtx, a.Theme)
			default:
				return a.LoginScreen.Layout(gtx, a.Theme)
			}
		}),
	)
}
