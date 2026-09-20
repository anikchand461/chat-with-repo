// Command mobile is the native Android (and desktop, for local testing)
// frontend for ChatWithRepo, built with Go + Gio. It talks only to the
// existing FastAPI backend over HTTPS — no local database, no bundled
// web assets.
package main

import (
	"embed"
	"os"

	"gioui.org/app"
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

// baseURL is the only thing you need to change to point this client
// at a different backend (e.g. a local dev server on 10.0.2.2 for the
// Android emulator, or 127.0.0.1 when running as a desktop binary).
const baseURL = "https://chat-with-repo-4vwy.onrender.com"

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
	app.Main()
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
	a := ui.NewApp(w, baseURL)
	a.Bootstrap()

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
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
