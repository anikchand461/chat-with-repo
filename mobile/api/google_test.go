package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestGoogleChallengeMatchesRFC7636(t *testing.T) {
	// Appendix B of RFC 7636.
	got := GoogleChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatal(got)
	}
	v := NewGoogleVerifier()
	if len(v) != 43 || v == NewGoogleVerifier() {
		t.Fatalf("verifier %q", v)
	}
}

func TestGoogleStartURLCarriesOnlyTheChallenge(t *testing.T) {
	v := NewGoogleVerifier()
	u, err := url.Parse(GoogleStartURL("https://api.example", v))
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/auth/google/mobile/start" || u.Query().Get("challenge") != GoogleChallenge(v) {
		t.Fatal(u)
	}
	if strings.Contains(u.String(), v) {
		t.Fatal("the verifier must never leave the app")
	}
}

func TestGoogleExchangeIsRetriedOnRender(t *testing.T) {
	fastTimers(t)
	var got googleExchangeRequest
	aws := newBackend(t, status(504))
	render := newBackend(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"access_token": "jwt"}`))
	})

	token, err := NewClient(aws.URL, render.URL).GoogleExchange("code", "verifier")
	if err != nil || token != "jwt" {
		t.Fatalf("token %q, err %v", token, err)
	}
	if got.Code != "code" || got.Verifier != "verifier" {
		t.Fatalf("sent %+v", got)
	}
}

func TestGoogleExchangeShowsTheBackendsMessage(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail": "Google sign-in expired. Please try again."}`))
	})
	_, err := NewClient(aws.URL, "").GoogleExchange("code", "verifier")
	if err == nil || err.Error() != "Google sign-in expired. Please try again." {
		t.Fatal(err)
	}
}

func TestBrowserBaseURL(t *testing.T) {
	fastTimers(t)
	render := newBackend(t, ok(`{}`))

	if got := NewClient(newBackend(t, ok(`{}`)).URL, render.URL); got.BrowserBaseURL() != got.BaseURL {
		t.Fatal("healthy AWS should be used")
	}
	for name, aws := range map[string]string{
		"throttled": newBackend(t, throttled).URL,
		"down":      deadURL(t),
	} {
		c := NewClient(aws, render.URL)
		if c.BrowserBaseURL() != render.URL || !c.primaryDown() {
			t.Fatalf("%s AWS: should use Render and remember AWS is down", name)
		}
	}
}

func TestGoogleVerifierIsUsedOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if TakeGoogleVerifier() != "" {
		t.Fatal("nothing saved yet")
	}
	if err := SaveGoogleVerifier("abc"); err != nil {
		t.Fatal(err)
	}
	if TakeGoogleVerifier() != "abc" || TakeGoogleVerifier() != "" {
		t.Fatal("the verifier must be returned once")
	}
}
