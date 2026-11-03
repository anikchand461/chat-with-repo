package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// AWS first, Render as fallback - the same rules as the web app's
// apiFetch (frontend/js/app.js).

var (
	// How long AWS gets to start answering (response headers) before we
	// fail over. A healthy request is never delayed by it, and once
	// headers arrive it no longer applies, so long streams are never cut off.
	primaryTimeout = 10 * time.Second

	// Once AWS fails, skip it for this long so only the first request pays
	// primaryTimeout.
	primarySkip = 5 * time.Minute

	// While AWS is skipped, check it in the background at most this often
	// so traffic returns to it as soon as it recovers.
	recheckEvery = 30 * time.Second

	// Render's free tier sleeps when idle (it normally is, since AWS takes
	// the traffic) and needs ~30-60 s to wake up, so requests sent there get
	// more time than the client's usual 30 s.
	fallbackTimeout = 90 * time.Second
)

// POSTs that are safe to send twice (same result if AWS already handled
// them). Every other POST - register, chat create, ask, checkout - is only
// re-sent to Render when AWS definitely never processed it.
var idempotentPosts = map[string]bool{
	"/auth/login":        true,
	"/auth/github-token": true,
}

func retrySafe(method, path string) bool {
	if method != http.MethodPost && method != http.MethodPatch {
		return true
	}
	return idempotentPosts[path]
}

// send performs the request against AWS first, falling back to Render once:
//   - no response headers within primaryTimeout, or a network/TLS failure:
//     retry-safe requests abandon AWS and go to Render. Other POSTs check
//     AWS health in parallel from the start: if the app is up it may already
//     be processing the request, so we keep waiting on AWS (or return its
//     error) instead of running it twice.
//   - 502/503 (the app is down behind AWS's front end): fall back.
//   - 504 (timeout - the app may still be working): retry-safe only.
//
// Any other status (incl. 4xx) is returned as-is. The response is returned
// as soon as headers arrive, so a stream that fails part-way is never
// restarted on Render. After a failover AWS is skipped for primarySkip.
func (c *Client) send(ctx context.Context, hc *http.Client, method, path string, body []byte, header http.Header) (*http.Response, error) {
	if c.Fallback == "" {
		return attempt(ctx, hc, c.BaseURL, method, path, body, header)
	}

	if c.primaryDown() {
		c.recheckPrimary()
		return attempt(ctx, fallbackClient(hc), c.Fallback, method, path, body, header)
	}

	safe := retrySafe(method, path)

	var alive func() bool
	if !safe {
		ch := make(chan bool, 1)
		go func() { ch <- c.backendUp(c.BaseURL) }()
		alive = sync.OnceValue(func() bool { return <-ch })
	}

	type result struct {
		resp *http.Response
		err  error
	}
	actx, cancel := context.WithCancel(ctx)
	done := make(chan result, 1)
	go func() {
		resp, err := attempt(actx, hc, c.BaseURL, method, path, body, header)
		done <- result{resp, err}
	}()

	timer := time.NewTimer(primaryTimeout)
	defer timer.Stop()

	var r result
	answered := false
	select {
	case r = <-done:
		answered = true
	case <-timer.C:
		if !safe && alive() {
			r = <-done
			answered = true
		}
	}

	if answered && r.err != nil {
		if !safe && alive() {
			cancel()
			return nil, r.err
		}
		answered = false
	}

	if !answered {
		cancel()
		// The abandoned attempt may still deliver a response; close it.
		go func() {
			if late := <-done; late.resp != nil {
				late.resp.Body.Close()
			}
		}()
		c.markPrimaryDown(true)
		return attempt(ctx, fallbackClient(hc), c.Fallback, method, path, body, header)
	}

	status := r.resp.StatusCode
	if status == http.StatusBadGateway || status == http.StatusServiceUnavailable ||
		(status == http.StatusGatewayTimeout && safe) {
		r.resp.Body.Close()
		cancel()
		c.markPrimaryDown(true)
		return attempt(ctx, fallbackClient(hc), c.Fallback, method, path, body, header)
	}

	r.resp.Body = cancelOnClose{r.resp.Body, cancel}
	return r.resp, nil
}

// fallbackClient is hc with room for Render to wake up (streams have no
// overall timeout and keep theirs).
func fallbackClient(hc *http.Client) *http.Client {
	if hc.Timeout == 0 || hc.Timeout >= fallbackTimeout {
		return hc
	}
	return &http.Client{Transport: hc.Transport, Timeout: fallbackTimeout}
}

func attempt(ctx context.Context, hc *http.Client, base, method, path string, body []byte, header http.Header) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header = header.Clone()
	return hc.Do(req)
}

// cancelOnClose releases the attempt's context once the caller is done
// with the body (cancelling it earlier would cut the body off).
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// backendUp reports whether the FastAPI app itself answers GET / within
// primaryTimeout.
func (c *Client) backendUp(base string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), primaryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (c *Client) primaryDown() bool {
	c.downMu.Lock()
	defer c.downMu.Unlock()
	return time.Now().Before(c.downUntil)
}

func (c *Client) markPrimaryDown(down bool) {
	c.downMu.Lock()
	defer c.downMu.Unlock()
	if down {
		c.downUntil = time.Now().Add(primarySkip)
	} else {
		c.downUntil = time.Time{}
	}
}

// recheckPrimary checks AWS in the background (one check at a time, at
// most every recheckEvery) and stops skipping it once it answers again.
// Nobody waits on it.
func (c *Client) recheckPrimary() {
	c.downMu.Lock()
	if c.rechecking || time.Since(c.lastRecheck) < recheckEvery {
		c.downMu.Unlock()
		return
	}
	c.rechecking = true
	c.lastRecheck = time.Now()
	c.downMu.Unlock()

	go func() {
		up := c.backendUp(c.BaseURL)
		c.downMu.Lock()
		c.rechecking = false
		if up {
			c.downUntil = time.Time{}
		}
		c.downMu.Unlock()
	}()
}
