// Package api is the HTTP client layer that talks to the existing
// ChatWithRepo FastAPI backend. It knows nothing about Gio or the UI —
// it only sends requests and returns typed results/errors.
package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
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
	http    *http.Client

	tokenMu sync.RWMutex // token is read by request goroutines and swapped on login/logout
	token   string
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
	c.tokenMu.Lock()
	c.token = token
	c.tokenMu.Unlock()
}

// Token returns the currently stored bearer token, if any.
func (c *Client) Token() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
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

	// The backend stores how long each answer took, so web and mobile show
	// the same timing. Meta is built from these fields.
	ResponseSeconds  *float64 `json:"response_seconds"`
	FirstWordSeconds *float64 `json:"first_word_seconds"`

	Meta *ResponseMeta `json:"-"`
}

func seconds(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }

// fillMeta turns the stored timing fields into Meta.
func (m *Message) fillMeta() {
	if m.Role != "assistant" || m.ResponseSeconds == nil {
		return
	}
	meta := ResponseMeta{Total: seconds(*m.ResponseSeconds)}
	if m.FirstWordSeconds != nil {
		meta.First = seconds(*m.FirstWordSeconds)
	}
	m.Meta = &meta
}

// ResponseMeta is the timing shown under an assistant answer.
type ResponseMeta struct {
	Total       time.Duration `json:"total"`
	First       time.Duration `json:"first"` // time to the first word; 0 if unknown
	Interrupted bool          `json:"interrupted,omitempty"`
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

// StreamResult is the outcome of AskStream. Tokens are delivered through
// the callback; this carries what can't be streamed.
type StreamResult struct {
	UpgradeRequired bool
	Reason          string
	Message         string

	Meta ResponseMeta
	// Streamed reports whether any answer text was received.
	Streamed bool
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
	if token := c.Token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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
	for i := range out {
		out[i].fillMeta()
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

// streamEvent is one `data:` line of /chat/{id}/ask/stream (SSE).
type streamEvent struct {
	Token string   `json:"token"`
	Done  bool     `json:"done"`
	Error string   `json:"error"`
	Total *float64 `json:"total"` // server-measured, sent with "done"
	First *float64 `json:"first"`
}

// AskStream is the streaming version of Ask: it calls onToken with each
// piece of the answer as the server produces it (POST /chat/{id}/ask/stream,
// server-sent events), so the UI can show text as it is written.
//
// Like Ask, a free-plan limit comes back as UpgradeRequired (a plain JSON
// body instead of an event stream) with no tokens delivered.
func (c *Client) AskStream(chatID, question string, onToken func(string)) (StreamResult, error) {
	var res StreamResult

	buf, err := json.Marshal(AskRequest{Question: question})
	if err != nil {
		return res, fmt.Errorf("encode request: %w", err)
	}

	// No overall client timeout: an answer may stream for a while. The
	// context bounds it instead.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/chat/"+chatID+"/ask/stream", bytes.NewReader(buf))
	if err != nil {
		return res, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if token := c.Token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()

	resp, err := (&http.Client{Transport: c.http.Transport}).Do(req)
	if err != nil {
		return res, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return res, ErrUnauthorized
	}

	// Errors and upgrade_required arrive as ordinary JSON.
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		raw, _ := io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var eb errorBody
			_ = json.Unmarshal(raw, &eb)
			if eb.Detail != "" {
				return res, errors.New(eb.Detail)
			}
			return res, fmt.Errorf("request failed (%d)", resp.StatusCode)
		}

		var ar AskResponse
		if err := json.Unmarshal(raw, &ar); err != nil {
			return res, fmt.Errorf("decode response: %w", err)
		}
		res.UpgradeRequired = ar.UpgradeRequired
		res.Reason = ar.Reason
		res.Message = ar.Message
		return res, nil
	}

	var (
		streamErr   string
		serverTotal *float64
		serverFirst *float64
	)
	reader := bufio.NewReader(resp.Body)

	for {
		line, readErr := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "data:") {
			var ev streamEvent
			if json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &ev) == nil {
				switch {
				case ev.Error != "":
					streamErr = ev.Error
				case ev.Done:
					if ev.Total != nil {
						serverTotal = ev.Total
						serverFirst = ev.First
					}
				case ev.Token != "":
					if !res.Streamed {
						res.Streamed = true
						res.Meta.First = time.Since(start)
					}
					onToken(ev.Token)
				}
			}
		}

		if readErr != nil {
			break
		}
	}

	res.Meta.Total = time.Since(start)

	// Prefer the server's timing so web and mobile agree.
	if serverTotal != nil {
		res.Meta.Total = seconds(*serverTotal)
		res.Meta.First = 0
		if serverFirst != nil {
			res.Meta.First = seconds(*serverFirst)
		}
	}

	switch {
	case streamErr != "" && !res.Streamed:
		return res, errors.New(streamErr)
	case streamErr != "":
		res.Meta.Interrupted = true
	}
	return res, nil
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
