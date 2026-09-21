package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAskStreamDeliversTokensAndTiming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/7/ask/stream" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		time.Sleep(50 * time.Millisecond)
		for _, tok := range []string{"Hel", "lo ", "world"} {
			fmt.Fprintf(w, "data: {\"token\": %q}\n\n", tok)
			f.Flush()
		}
		fmt.Fprint(w, "data: {\"done\": true, \"total\": 4.2, \"first\": 1.8}\n\n")
	}))
	defer srv.Close()

	var got []string
	res, err := NewClient(srv.URL).AskStream("7", "hi", func(s string) { got = append(got, s) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "") != "Hello world" || !res.Streamed {
		t.Fatalf("tokens = %q streamed=%v", got, res.Streamed)
	}
	// Server-measured timing wins over the local clock.
	if res.Meta.Total != 4200*time.Millisecond || res.Meta.First != 1800*time.Millisecond {
		t.Fatalf("want server timing 4.2s/1.8s, got %+v", res.Meta)
	}
}

func TestMessagesCarryStoredTiming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"role":"user","content":"q","response_seconds":null,"first_word_seconds":null},
			{"role":"assistant","content":"a","response_seconds":63.4,"first_word_seconds":56.1},
			{"role":"assistant","content":"old","response_seconds":null,"first_word_seconds":null}]`)
	}))
	defer srv.Close()

	msgs, err := NewClient(srv.URL).Messages("1")
	if err != nil {
		t.Fatal(err)
	}
	if msgs[0].Meta != nil || msgs[2].Meta != nil {
		t.Fatal("messages without stored timing must have no Meta")
	}
	if m := msgs[1].Meta; m == nil || m.Total != seconds(63.4) || m.First != seconds(56.1) {
		t.Fatalf("meta = %+v", msgs[1].Meta)
	}
}

func TestAskStreamUpgradeRequiredAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/chat/1/ask/stream" {
			fmt.Fprint(w, `{"upgrade_required":true,"reason":"daily_questions","message":"limit"}`)
			return
		}
		w.WriteHeader(500)
		fmt.Fprint(w, `{"detail":"boom"}`)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	res, err := c.AskStream("1", "q", func(string) { t.Error("no tokens expected") })
	if err != nil || !res.UpgradeRequired || res.Reason != "daily_questions" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, err := c.AskStream("2", "q", func(string) {}); err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}

func TestAskStreamMidStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"token\": \"partial\"}\n\ndata: {\"error\": \"llm failed\"}\n\n")
	}))
	defer srv.Close()

	res, err := NewClient(srv.URL).AskStream("1", "q", func(string) {})
	if err != nil || !res.Meta.Interrupted {
		t.Fatalf("want interrupted partial answer, got res=%+v err=%v", res, err)
	}
}
