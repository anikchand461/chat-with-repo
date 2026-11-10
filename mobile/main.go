// Command mobile is the native Android (and desktop, for local testing)
// frontend for ChatWithRepo, built with Go + Gio. It talks only to the
// existing FastAPI backend over HTTPS — no local database, no bundled
// web assets.
package main

import (
	"embed"
	"os"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"chatwithrepo/mobile/ui"
)

// assets holds the images shown in the UI (assets/icon.png and
// assets/github.png). Embedding them bundles the files into the binary,
// so they load the same way on desktop and in the Android build.
//
//go:embed assets/*
var assets embed.FS

// AWS is the primary backend. Render is only used when AWS is unreachable
// or reports the app as down - same rules as the web app (see
// api/failover.go).
const (
	// apiPrimary  = "http://127.0.0.1:8000"
	apiPrimary  = "https://qettzfl73r7xhbres3oqvx3qt40mmqrp.lambda-url.ap-south-1.on.aws"
	apiFallback = "https://chat-with-repo-4vwy.onrender.com"
)

// Local testing only (no fallback): use these instead - 127.0.0.1 for the
// desktop binary, 10.0.2.2 for the Android emulator.
// const apiPrimary = "http://127.0.0.1:8000"
// const apiFallback = ""

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("ChatWithRepo"),
			app.Size(unit.Dp(380), unit.Dp(760)),
			// Android: bars take the app's dark green and get light icons.
			app.StatusColor(ui.StatusBarColor),
			app.NavigationColor(ui.NavigationBarColor),
		)
		if err := run(w); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}()
	// Replaces app.Main: also delivers links that open the app - the
	// browser returning from Google sign-in (chatwithrepo://auth, the
	// APK is built with `gogio -schemes chatwithrepo`).
	app.Events(func(e event.Event) bool {
		if e, ok := e.(app.URLEvent); ok {
			ui.DeliverURL(e.URL)
		}
		return true
	})
}

func run(w *app.Window) error {
	if b, err := assets.ReadFile("assets/fonts/NotoColorEmoji.ttf"); err == nil {
		ui.SetEmojiFont(b)
	}
	if b, err := assets.ReadFile("assets/icon.png"); err == nil {
		ui.SetLogo(b)
	}
	if b, err := assets.ReadFile("assets/github.png"); err == nil {
		ui.SetGitHubIcon(b)
	}
	if b, err := assets.ReadFile("assets/profile.png"); err == nil {
		ui.SetProfileIcon(b)
	}
	if b, err := assets.ReadFile("assets/logout.png"); err == nil {
		ui.SetLogoutIcon(b)
	}
	if b, err := assets.ReadFile("assets/manual.png"); err == nil {
		ui.SetManualIcon(b)
	}
	if b, err := assets.ReadFile("assets/web.png"); err == nil {
		ui.SetWebIcon(b)
	}
	if b, err := assets.ReadFile("assets/google.png"); err == nil {
		ui.SetGoogleIcon(b)
	}
	a := ui.NewApp(w, apiPrimary, apiFallback)
	a.Bootstrap()

	var ops op.Ops
	for {
		ev := w.Event()
		a.ListenEvents(ev)
		switch e := ev.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			// Draw across the whole window (behind the status and
			// navigation bars) and hand the insets to the UI, which
			// keeps only its content clear of them. app.NewContext
			// would shrink the drawing area and leave the bars bare.
			ops.Reset()
			gtx := layout.Context{
				Ops:         &ops,
				Now:         e.Now,
				Source:      e.Source,
				Metric:      e.Metric,
				Constraints: layout.Exact(e.Size),
			}
			ui.SetInsets(e.Insets)
			a.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
