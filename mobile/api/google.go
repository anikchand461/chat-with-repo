package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
)

// Google sign-in for the app (backend/api/auth.py, /auth/google/mobile/*):
// the system browser runs Google's page, the backend verifies the account
// and sends the browser back to chatwithrepo://auth with a short-lived code,
// and the app swaps that code - plus the secret verifier only it knows
// (PKCE, RFC 8252) - for the usual access token.

// GoogleReturnScheme is the URL scheme the backend sends the browser back
// to. The APK must be built with `gogio -schemes chatwithrepo`.
const GoogleReturnScheme = "chatwithrepo"

// NewGoogleVerifier returns a fresh random PKCE verifier (43 chars).
func NewGoogleVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// GoogleChallenge is the PKCE S256 challenge for verifier.
func GoogleChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GoogleStartURL is the page to open in the browser. base is the backend
// to use for the whole browser part (see BrowserBaseURL).
func GoogleStartURL(base, verifier string) string {
	return base + "/auth/google/mobile/start?" + url.Values{"challenge": {GoogleChallenge(verifier)}}.Encode()
}

type googleExchangeRequest struct {
	Code     string `json:"code"`
	Verifier string `json:"verifier"`
}

// GoogleExchange trades the code from the chatwithrepo://auth link and the
// verifier for an access token. Any backend can redeem it (the code is
// signed with the shared JWT secret), so normal failover applies.
func (c *Client) GoogleExchange(code, verifier string) (string, error) {
	var out AuthResponse
	err := c.do(http.MethodPost, "/auth/google/mobile/exchange", googleExchangeRequest{Code: code, Verifier: verifier}, &out)
	if err != nil {
		return "", err
	}
	return out.AccessToken, nil
}
