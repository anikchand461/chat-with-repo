// Package api is the HTTP client layer that talks to the existing
// ChatWithRepo FastAPI backend. It knows nothing about Gio or the UI —
// it only sends requests and returns typed results/errors.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrUnauthorized is returned whenever the backend responds with 401.
// The UI layer watches for this to send the user back to the Login screen.
var ErrUnauthorized = errors.New("unauthorized")

// Client is a small wrapper around net/http configured for the
// ChatWithRepo API. BaseURL is the only thing you need to change to
// point the app at a different backend (e.g. local dev).
type Client struct {
	BaseURL string
	token   string
	http    *http.Client
}

// NewClient builds a Client pointed at baseURL (no trailing slash expected).
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetToken stores the bearer token used for all subsequent requests.
func (c *Client) SetToken(token string) {
	c.token = token
}

// Token returns the currently stored bearer token, if any.
func (c *Client) Token() string {
	return c.token
}

// ---- wire types (mirror the FastAPI response/request bodies) ----

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
}

type MeResponse struct {
	Email          string `json:"email"`
	HasGithubToken bool   `json:"has_github_token"`
}

type Chat struct {
	ChatID ChatID `json:"chat_id"`
	Title  string `json:"title"`
	Branch string `json:"branch"`
}

type CreateChatRequest struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

// CreateChatResponse mirrors what the FastAPI backend returns from
// POST /chat/create. On the free plan, once the repo-chat limit is
// hit the backend still answers 200 OK but sets UpgradeRequired
// instead of a usable ChatID — the same "soft failure" shape used by
// the web frontend (see js/app.js, data.upgrade_required).
type CreateChatResponse struct {
	ChatID          ChatID `json:"chat_id"`
	UpgradeRequired bool   `json:"upgrade_required"`
	Reason          string `json:"reason"`
	Message         string `json:"message"`
}

// ChatID represents chat_id as sent by the backend, which returns it
// as a JSON number. We keep it as a string throughout the rest of the
// app (it's only ever used as an opaque identifier in URLs), but
// unmarshal leniently from either a number or a string so a future
// backend change to string IDs won't break this client either.
type ChatID string

func (c *ChatID) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	*c = ChatID(s)
	return nil
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AskRequest struct {
	Question string `json:"question"`
}

// AskResponse mirrors POST /chat/{id}/ask. Like CreateChatResponse,
// a free-plan daily question limit comes back as 200 OK with
// UpgradeRequired set rather than an Answer.
type AskResponse struct {
	Answer          string `json:"answer"`
	UpgradeRequired bool   `json:"upgrade_required"`
	Reason          string `json:"reason"`
	Message         string `json:"message"`
}

// PaymentStatus mirrors GET /payment/status.
type PaymentStatus struct {
	Plan  string `json:"plan"`
	IsPro bool   `json:"is_pro"`
}

// CheckoutResponse mirrors POST /payment/checkout: a URL to the
// hosted payment page the web frontend redirects the browser to.
type CheckoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
}

type errorBody struct {
	Detail string `json:"detail"`
}

// ---- internal request helper ----

// do performs an HTTP request against path, optionally sending body as
// JSON, and decodes a successful JSON response into out (if non-nil).
func (c *Client) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var eb errorBody
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &eb)
		}
		if eb.Detail != "" {
			return errors.New(eb.Detail)
		}
		return fmt.Errorf("request failed (%d)", resp.StatusCode)
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// ---- public API ----

func (c *Client) Login(email, password string) (string, error) {
	var out AuthResponse
	err := c.do(http.MethodPost, "/auth/login", AuthRequest{Email: email, Password: password}, &out)
	if err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func (c *Client) Register(email, password string) (string, error) {
	var out AuthResponse
	err := c.do(http.MethodPost, "/auth/register", AuthRequest{Email: email, Password: password}, &out)
	if err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func (c *Client) Me() (*MeResponse, error) {
	var out MeResponse
	if err := c.do(http.MethodGet, "/auth/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListChats() ([]Chat, error) {
	var out []Chat
	if err := c.do(http.MethodGet, "/chat/list", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateChat returns the full response so callers can check
// UpgradeRequired before treating the call as a success — a 200
// response does not by itself mean a chat was created.
func (c *Client) CreateChat(owner, repo, branch string) (CreateChatResponse, error) {
	var out CreateChatResponse
	err := c.do(http.MethodPost, "/chat/create", CreateChatRequest{
		Owner: owner, Repo: repo, Branch: branch,
	}, &out)
	if err != nil {
		return CreateChatResponse{}, err
	}
	return out, nil
}

func (c *Client) Messages(chatID string) ([]Message, error) {
	var out []Message
	if err := c.do(http.MethodGet, "/chat/"+chatID+"/messages", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Ask returns the full response so callers can check UpgradeRequired
// (e.g. the free plan's daily question limit) before treating the
// call as a successful answer.
func (c *Client) Ask(chatID, question string) (AskResponse, error) {
	var out AskResponse
	err := c.do(http.MethodPost, "/chat/"+chatID+"/ask", AskRequest{Question: question}, &out)
	if err != nil {
		return AskResponse{}, err
	}
	return out, nil
}

// PaymentStatus reports whether the current user is on the free plan
// or Pro.
func (c *Client) PaymentStatus() (PaymentStatus, error) {
	var out PaymentStatus
	if err := c.do(http.MethodGet, "/payment/status", nil, &out); err != nil {
		return PaymentStatus{}, err
	}
	return out, nil
}

// CreateCheckout starts a Pro upgrade and returns the URL of the
// hosted payment page to open (the same one the web frontend's
// "Upgrade to Pro" button redirects to).
func (c *Client) CreateCheckout() (string, error) {
	var out CheckoutResponse
	if err := c.do(http.MethodPost, "/payment/checkout", nil, &out); err != nil {
		return "", err
	}
	return out.CheckoutURL, nil
}
