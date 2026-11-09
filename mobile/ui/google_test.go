package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"gioui.org/app"

	"chatwithrepo/mobile/api"
)

func googleTestApp(t *testing.T, exchange http.HandlerFunc) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // SaveToken/verifier files must not touch real data
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/google/mobile/exchange":
			exchange(w, r)
		case "/chat/list":
			w.Write([]byte(`[]`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return NewApp(new(app.Window), srv.URL, "")
}

func returnLink(query string) *url.URL {
	u, _ := url.Parse("chatwithrepo://auth?" + query)
	return u
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal("timed out waiting for " + what)
}

func TestGoogleReturnSignsIn(t *testing.T) {
	var sent map[string]string
	a := googleTestApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&sent)
		w.Write([]byte(`{"access_token": "google-jwt"}`))
	})
	if err := api.SaveGoogleVerifier("my-verifier"); err != nil {
		t.Fatal(err)
	}

	a.handleURL(returnLink("code=one-time"))
	waitFor(t, "the dashboard", func() bool { return a.currentScreen() == ScreenDashboard })

	if a.Client.Token() != "google-jwt" || sent["code"] != "one-time" || sent["verifier"] != "my-verifier" {
		t.Fatalf("token %q, sent %v", a.Client.Token(), sent)
	}
	if saved, _ := api.LoadToken(); saved != "google-jwt" {
		t.Fatal("the session should be saved like a normal login")
	}
	if api.TakeGoogleVerifier() != "" {
		t.Fatal("the verifier must be gone after use")
	}
}

func TestGoogleReturnWithErrorShowsIt(t *testing.T) {
	a := googleTestApp(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not exchange") })
	api.SaveGoogleVerifier("v")

	a.handleURL(returnLink("error=" + url.QueryEscape("An account with this email already exists.")))

	if _, msg := a.LoginScreen.snapshot(); msg != "An account with this email already exists." {
		t.Fatalf("error %q", msg)
	}
	if a.currentScreen() != ScreenLogin || api.TakeGoogleVerifier() != "" {
		t.Fatal("should stay on Login and drop the verifier")
	}
}

func TestGoogleReturnWithoutAPendingSignIn(t *testing.T) {
	a := googleTestApp(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not exchange") })
	a.ShowRegister()

	a.handleURL(returnLink("code=one-time")) // no verifier saved: not started here, or already used

	if _, msg := a.RegisterScreen.snapshot(); msg != "Google sign-in expired. Please try again." {
		t.Fatalf("error %q", msg)
	}
}

func TestGoogleReturnExchangeFails(t *testing.T) {
	a := googleTestApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail": "Google sign-in expired. Please try again."}`))
	})
	api.SaveGoogleVerifier("v")

	a.handleURL(returnLink("code=old"))
	waitFor(t, "the error", func() bool { _, msg := a.LoginScreen.snapshot(); return msg != "" })

	if a.currentScreen() != ScreenLogin || a.Client.Token() != "" || a.GoogleBusy() {
		t.Fatal("should stay signed out on Login, ready to try again")
	}
}

func TestOtherLinksAreIgnored(t *testing.T) {
	a := googleTestApp(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not exchange") })
	api.SaveGoogleVerifier("v")
	for _, raw := range []string{"https://auth?code=x", "chatwithrepo://other?code=x"} {
		u, _ := url.Parse(raw)
		a.handleURL(u)
	}
	if api.TakeGoogleVerifier() != "v" {
		t.Fatal("unrelated links must not use up a pending sign-in")
	}
}
