package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// backend is a fake API server that counts the requests it receives.
type backend struct {
	*httptest.Server
	calls atomic.Int32 // requests other than the GET / health check
}

func newBackend(t *testing.T, handler http.HandlerFunc) *backend {
	b := &backend{}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			b.calls.Add(1)
		}
		handler(w, r)
	}))
	t.Cleanup(b.Close)
	return b
}

func ok(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(code)
	}
}

// slow answers everything after delay (the health check too, unless
// healthy is set).
func slow(delay time.Duration, healthy bool, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !(healthy && r.URL.Path == "/") {
			time.Sleep(delay)
		}
		fmt.Fprint(w, body)
	}
}

// deadURL is an address nothing listens on (connection refused).
func deadURL(t *testing.T) string {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}

func fastTimers(t *testing.T) {
	timeout, skip, recheck := primaryTimeout, primarySkip, recheckEvery
	primaryTimeout, primarySkip, recheckEvery = 200*time.Millisecond, time.Minute, 0
	t.Cleanup(func() { primaryTimeout, primarySkip, recheckEvery = timeout, skip, recheck })
}

func TestTimeoutIs10SecondsAndAWSIsSkippedFor5Minutes(t *testing.T) {
	if primaryTimeout != 10*time.Second || primarySkip != 5*time.Minute {
		t.Fatalf("timeout %v, skip %v", primaryTimeout, primarySkip)
	}
}

func TestPrimaryAnswersNoFallback(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, ok(`[]`))
	render := newBackend(t, ok(`[]`))

	if _, err := NewClient(aws.URL, render.URL).ListChats(); err != nil {
		t.Fatal(err)
	}
	if aws.calls.Load() != 1 || render.calls.Load() != 0 {
		t.Fatalf("aws %d, render %d", aws.calls.Load(), render.calls.Load())
	}
}

func TestClientErrorsAreReturnedNotFailedOver(t *testing.T) {
	fastTimers(t)
	for _, code := range []int{400, 401, 403, 404, 409} {
		aws := newBackend(t, status(code))
		render := newBackend(t, ok(`[]`))

		_, err := NewClient(aws.URL, render.URL).ListChats()
		if err == nil || render.calls.Load() != 0 {
			t.Fatalf("%d: err %v, render calls %d", code, err, render.calls.Load())
		}
	}
}

func TestUnreachablePrimaryFailsOverAndIsSkipped(t *testing.T) {
	fastTimers(t)
	render := newBackend(t, ok(`[]`))
	c := NewClient(deadURL(t), render.URL)

	if _, err := c.ListChats(); err != nil {
		t.Fatal(err)
	}
	if !c.primaryDown() {
		t.Fatal("AWS should be marked down")
	}

	start := time.Now()
	if _, err := c.ListChats(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("second request should go straight to Render")
	}
	if render.calls.Load() != 2 {
		t.Fatalf("render calls %d", render.calls.Load())
	}
}

func TestPrimaryDownStatusesFailOver(t *testing.T) {
	fastTimers(t)
	for _, code := range []int{502, 503, 504} {
		aws := newBackend(t, status(code))
		render := newBackend(t, ok(`[]`))

		if _, err := NewClient(aws.URL, render.URL).ListChats(); err != nil {
			t.Fatalf("%d: %v", code, err)
		}
		if render.calls.Load() != 1 {
			t.Fatalf("%d: render calls %d", code, render.calls.Load())
		}
	}
}

func Test504OnACreateIsNotRepeatedOnRender(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, status(504))
	render := newBackend(t, ok(`{"chat_id": 1}`))

	if _, err := NewClient(aws.URL, render.URL).CreateChat("o", "r", "main"); err == nil {
		t.Fatal("expected the 504 error")
	}
	if render.calls.Load() != 0 {
		t.Fatal("create must not be sent to Render after a 504")
	}
}

func TestSlowPrimaryFailsOverForReads(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, slow(time.Second, false, `[]`))
	render := newBackend(t, ok(`[]`))

	start := time.Now()
	if _, err := NewClient(aws.URL, render.URL).ListChats(); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 600*time.Millisecond {
		t.Fatalf("waited %v for a slow AWS", took)
	}
	if render.calls.Load() != 1 {
		t.Fatalf("render calls %d", render.calls.Load())
	}
}

func TestSlowCreateWaitsForALivePrimary(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, slow(500*time.Millisecond, true, `{"chat_id": 7, "indexing": true}`))
	render := newBackend(t, ok(`{"chat_id": 99}`))

	res, err := NewClient(aws.URL, render.URL).CreateChat("o", "r", "main")
	if err != nil {
		t.Fatal(err)
	}
	if res.ChatID != "7" || render.calls.Load() != 0 {
		t.Fatalf("chat %s, render calls %d - must not run twice", res.ChatID, render.calls.Load())
	}
}

func TestSlowCreateOnADeadPrimaryFailsOver(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, slow(time.Second, false, `{"chat_id": 7}`))
	render := newBackend(t, ok(`{"chat_id": 99}`))

	res, err := NewClient(aws.URL, render.URL).CreateChat("o", "r", "main")
	if err != nil {
		t.Fatal(err)
	}
	if res.ChatID != "99" {
		t.Fatalf("chat %s", res.ChatID)
	}
}

func TestStreamBrokenPartWayIsNotRestartedOnRender(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"token\": \"Hel\"}\n\n")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler) // connection drops mid-answer
	})
	render := newBackend(t, ok(`{}`))

	var got strings.Builder
	res, _ := NewClient(aws.URL, render.URL).AskStream("1", "q", func(s string) { got.WriteString(s) }, nil)
	if got.String() != "Hel" || !res.Streamed {
		t.Fatalf("got %q", got.String())
	}
	if render.calls.Load() != 0 {
		t.Fatal("a started answer must not be asked again on Render")
	}
}

func TestLongStreamIsNotCutOffByTheTimeout(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		for _, tok := range []string{"a", "b", "c"} {
			fmt.Fprintf(w, "data: {\"token\": %q}\n\n", tok)
			f.Flush()
			time.Sleep(150 * time.Millisecond) // 450 ms in total, over primaryTimeout
		}
		fmt.Fprint(w, "data: {\"done\": true}\n\n")
	})
	render := newBackend(t, ok(`{}`))

	var got strings.Builder
	if _, err := NewClient(aws.URL, render.URL).AskStream("1", "q", func(s string) { got.WriteString(s) }, nil); err != nil {
		t.Fatal(err)
	}
	if got.String() != "abc" || render.calls.Load() != 0 {
		t.Fatalf("got %q, render calls %d", got.String(), render.calls.Load())
	}
}

func TestTrafficReturnsToPrimaryAfterRecovery(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, ok(`[]`))
	render := newBackend(t, ok(`[]`))
	c := NewClient(aws.URL, render.URL)
	c.markPrimaryDown(true)

	c.ListChats() // goes to Render, starts a background check of AWS
	deadline := time.Now().Add(2 * time.Second)
	for c.primaryDown() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if c.primaryDown() {
		t.Fatal("AWS should be back after the recheck")
	}

	c.ListChats()
	if aws.calls.Load() != 1 || render.calls.Load() != 1 {
		t.Fatalf("aws %d, render %d", aws.calls.Load(), render.calls.Load())
	}
}

func TestNoFallbackConfigured(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, status(503))

	if _, err := NewClient(aws.URL, "").ListChats(); err == nil {
		t.Fatal("expected the 503 error")
	}
	if aws.calls.Load() != 1 {
		t.Fatalf("aws calls %d", aws.calls.Load())
	}
}

func TestLoginIsRetriedOnRenderAfterA504(t *testing.T) {
	fastTimers(t)
	aws := newBackend(t, status(504))
	render := newBackend(t, ok(`{"access_token": "t"}`))

	token, err := NewClient(aws.URL, render.URL).Login("a@b.c", "pw")
	if err != nil || token != "t" {
		t.Fatalf("token %q, err %v", token, err)
	}
}
