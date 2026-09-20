// Package ui contains every screen of the ChatWithRepo Android client
// plus the small App shell that switches between them. There is no
// heavyweight framework here: App just holds "which screen is active"
// and delegates Layout to that screen every frame.
package ui

import (
	"image"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
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
	prev   Screen // screen shown before the latest navigation
	navSeq int    // bumped on every navigation, including chat -> chat

	// gen is the session generation. It is bumped whenever the account
	// changes (logout, 401, new login). Async work captures it when it
	// starts and drops its result if it no longer matches, so a slow
	// response from a previous account can never touch the current one.
	gen uint64

	// Page-transition state (UI goroutine only).
	shownSeq   int
	transStart time.Time
	transDir   int // +1 forward (slide in from right), -1 back, 0 fade only

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
	gen := a.Generation()
	a.Client.SetToken(token)
	go func() {
		_, err := a.Client.Me()
		if !a.IsCurrent(gen) {
			return // the user logged out/in meanwhile: don't touch the new session
		}
		if err != nil {
			_ = api.ClearToken()
			a.Client.SetToken("")
			return
		}
		a.ShowDashboard()
	}()
}

// Generation returns the current session generation.
func (a *App) Generation() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.gen
}

// IsCurrent reports whether gen still identifies the active session.
func (a *App) IsCurrent(gen uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.gen == gen
}

// resetSession invalidates every in-flight request (by bumping the
// generation) and then wipes all account-specific state. Order matters:
// the bump comes first so no old goroutine can write after the wipe.
func (a *App) resetSession() {
	a.mu.Lock()
	a.gen++
	a.mu.Unlock()

	a.Dashboard.reset()
	a.Chat.reset()
	a.LoginScreen.reset()
	a.RegisterScreen.reset()
}

// BeginSession starts a fresh session for a successful login/registration:
// new generation, empty per-account state, then the dashboard (showing its
// loading state until the new account's data arrives).
func (a *App) BeginSession(token string) {
	a.resetSession()
	a.Client.SetToken(token)
	_ = api.SaveToken(token)
	a.ShowDashboard()
}

func (a *App) currentScreen() Screen {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.screen
}

func (a *App) setScreen(s Screen) {
	a.mu.Lock()
	a.prev = a.screen
	a.screen = s
	a.navSeq++
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
	// Reload marks the dashboard as loading synchronously; do it before
	// the switch so the first frame shows the loading UI, never an empty
	// or stale list.
	a.Dashboard.Reload()
	a.setScreen(ScreenDashboard)
}

// ShowChat switches to the Chat screen for the given chat and loads
// its message history.
func (a *App) ShowChat(chatID, title, branch string) {
	a.Chat.Open(chatID, title, branch)
	a.setScreen(ScreenChat)
}

// HandleUnauthorized is called whenever an API call comes back with a
// 401. It clears the session and sends the user back to Login. Callers
// must first check IsCurrent so a stale response can't log out a newer
// session.
func (a *App) HandleUnauthorized() {
	a.endSession()
}

// Logout clears the session explicitly (user tapped Logout).
func (a *App) Logout() {
	a.endSession()
}

func (a *App) endSession() {
	a.resetSession()
	_ = api.ClearToken()
	a.Client.SetToken("")
	a.ShowLogin() // only after the old state is gone
}

// Layout draws whichever screen is currently active. This is called
// once per frame from main.go.
func (a *App) Layout(gtx layout.Context) layout.Dimensions {
	const duration = 200 * time.Millisecond

	a.mu.Lock()
	seq, prev, cur := a.navSeq, a.prev, a.screen
	a.mu.Unlock()
	if seq != a.shownSeq {
		a.shownSeq = seq
		a.transStart = gtx.Now
		switch {
		case cur > prev:
			a.transDir = 1
		case cur < prev:
			a.transDir = -1
		default:
			a.transDir = 0
		}
	}

	// The incoming screen fades in while sliding a short distance from
	// the side it "comes from". Only frames inside the window are
	// requested, so an idle app never redraws.
	p := float32(1)
	if el := gtx.Now.Sub(a.transStart); el < duration {
		p = float32(el) / float32(duration)
		gtx.Execute(op.InvalidateCmd{})
	}
	e := 1 - (1-p)*(1-p)*(1-p) // ease-out cubic

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return fill(gtx, authBgBottom)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			if p < 1 {
				defer paint.PushOpacity(gtx.Ops, e).Pop()
				dx := int(float32(a.transDir*gtx.Dp(unit.Dp(28))) * (1 - e))
				defer op.Offset(image.Pt(dx, 0)).Push(gtx.Ops).Pop()
			}
			switch cur {
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
