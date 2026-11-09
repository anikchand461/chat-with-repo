package ui

import (
	"net/url"
	"runtime"

	"chatwithrepo/mobile/api"
)

// Google sign-in (see api/google.go): the button opens the system browser,
// and the backend sends the browser back to chatwithrepo://auth, which
// Android delivers to main as an app.URLEvent and main passes to DeliverURL.

// googleSignInAvailable: only the APK registers the chatwithrepo:// scheme
// (gogio -schemes), so a desktop build could never be sent back.
var googleSignInAvailable = runtime.GOOS == "android"

// incomingURLs carries links from main's event loop to the App, which may
// not exist yet when Android starts the app from a link.
var incomingURLs = make(chan *url.URL, 4)

// DeliverURL hands a link that opened the app to the UI. Called from main.
func DeliverURL(u *url.URL) {
	select {
	case incomingURLs <- u:
	default: // nobody is reading and the buffer is full: drop it
	}
}

func (a *App) watchURLs() {
	for u := range incomingURLs {
		a.handleURL(u)
	}
}

// GoogleBusy reports whether a Google sign-in is being started or finished.
func (a *App) GoogleBusy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.googleBusy
}

// setGoogleBusy updates the flag; turning it on fails if it already was.
func (a *App) setGoogleBusy(v bool) bool {
	a.mu.Lock()
	if v && a.googleBusy {
		a.mu.Unlock()
		return false
	}
	a.googleBusy = v
	a.mu.Unlock()
	if a.Window != nil {
		a.Window.Invalidate()
	}
	return true
}

// StartGoogleSignIn opens Google's sign-in page in the browser.
func (a *App) StartGoogleSignIn() {
	if !a.setGoogleBusy(true) {
		return
	}
	a.authError("")
	go func() {
		// Not busy once the browser is open: the user may come back
		// without finishing and should be able to try again.
		defer a.setGoogleBusy(false)

		verifier := api.NewGoogleVerifier()
		base := a.Client.BrowserBaseURL() // may wait for the AWS health check
		if err := api.SaveGoogleVerifier(verifier); err != nil {
			a.authError("Could not start Google sign-in. Please try again.")
			return
		}
		if err := openURL(api.GoogleStartURL(base, verifier)); err != nil {
			a.authError("Could not open the browser for Google sign-in.")
		}
	}()
}

// handleURL finishes a Google sign-in when the browser sends the user back.
func (a *App) handleURL(u *url.URL) {
	if u == nil || u.Scheme != api.GoogleReturnScheme || u.Host != "auth" {
		return
	}
	q := u.Query()
	verifier := api.TakeGoogleVerifier()
	if msg := q.Get("error"); msg != "" {
		a.authError(msg)
		return
	}
	code := q.Get("code")
	if code == "" || verifier == "" {
		a.authError("Google sign-in expired. Please try again.")
		return
	}
	if !a.setGoogleBusy(true) {
		return
	}

	gen := a.Generation()
	go func() {
		token, err := a.Client.GoogleExchange(code, verifier)
		a.setGoogleBusy(false)
		if !a.IsCurrent(gen) {
			return
		}
		if err != nil {
			a.authError(err.Error())
			return
		}
		a.BeginSession(token)
	}()
}

// authError shows msg on the Login or Register screen, whichever is open.
func (a *App) authError(msg string) {
	switch a.currentScreen() {
	case ScreenRegister:
		a.RegisterScreen.setError(msg)
	case ScreenLogin:
		a.LoginScreen.setError(msg)
	}
}
