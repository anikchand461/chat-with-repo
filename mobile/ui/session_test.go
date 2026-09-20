package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gioui.org/app"
)

// TestNoCrossAccountLeak logs Account A in, starts a slow dashboard load,
// logs out and immediately logs Account B in. A's late response must never
// reach the UI state.
func TestNoCrossAccountLeak(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // SaveToken/ClearToken must not touch real data
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/chat/list":
			if auth == "Bearer tokA" {
				time.Sleep(300 * time.Millisecond) // slow: outlives the logout
				json.NewEncoder(w).Encode([]map[string]string{{"chat_id": "a1", "title": "A/private", "branch": "main"}})
				return
			}
			json.NewEncoder(w).Encode([]map[string]string{{"chat_id": "b1", "title": "B/repo", "branch": "main"}})
		case "/auth/me":
			if auth == "Bearer tokA" {
				json.NewEncoder(w).Encode(map[string]any{"email": "a@x.com"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"email": "b@x.com"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"is_pro": false})
		}
	}))
	defer srv.Close()

	a := NewApp(new(app.Window), srv.URL)

	for round := 0; round < 5; round++ {
		a.BeginSession("tokA") // Account A logs in, dashboard starts loading
		time.Sleep(50 * time.Millisecond)
		a.Logout()
		a.BeginSession("tokB") // immediately Account B

		// Right after login: nothing of A may be visible, only loading.
		snap := a.Dashboard.snapshot()
		if len(snap.chats) != 0 || snap.email != "" || !snap.loading {
			t.Fatalf("round %d: stale/empty state after login: %+v", round, snap)
		}

		time.Sleep(500 * time.Millisecond) // A's slow response has now arrived
		snap = a.Dashboard.snapshot()
		if len(snap.chats) != 1 || snap.chats[0].Title != "B/repo" || snap.email != "b@x.com" {
			t.Fatalf("round %d: wrong data after B loaded: %+v", round, snap)
		}

		// Reverse direction: B logs out, A logs in immediately.
		a.Logout()
		a.BeginSession("tokB")
		time.Sleep(20 * time.Millisecond)
		a.Logout()
		snap = a.Dashboard.snapshot()
		if len(snap.chats) != 0 || snap.email != "" {
			t.Fatalf("round %d: data survived logout: %+v", round, snap)
		}
		time.Sleep(100 * time.Millisecond)
		snap = a.Dashboard.snapshot()
		if len(snap.chats) != 0 {
			t.Fatalf("round %d: late response repopulated a logged-out dashboard: %+v", round, snap)
		}
	}
}
