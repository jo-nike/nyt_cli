// Package client is a thin HTTP client for the New York Times developer APIs.
//
// All NYT APIs share the same host (api.nytimes.com), authenticate with an
// `api-key` query parameter, and return JSON. This package centralizes URL
// construction, authentication, rate-limit handling (HTTP 429 + Retry-After),
// retries on transient failures, and error decoding so the individual command
// implementations stay small.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the common host for every NYT API.
const DefaultBaseURL = "https://api.nytimes.com"

// Client performs authenticated requests against the NYT APIs.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTP       *http.Client
	UserAgent  string
	MaxRetries int
	// MinInterval, when > 0, enforces a minimum delay between successive
	// requests so callers stay under the NYT per-second/per-minute caps.
	MinInterval time.Duration
	Verbose     bool

	mu       sync.Mutex
	lastCall time.Time
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the underlying *http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.HTTP = h } }

// WithBaseURL overrides the API host (useful for testing).
func WithBaseURL(u string) Option { return func(c *Client) { c.BaseURL = u } }

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option { return func(c *Client) { c.UserAgent = ua } }

// WithTimeout sets the per-request timeout on the default HTTP client.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if c.HTTP == nil {
			c.HTTP = &http.Client{}
		}
		c.HTTP.Timeout = d
	}
}

// WithMinInterval throttles requests so at least d elapses between them.
func WithMinInterval(d time.Duration) Option { return func(c *Client) { c.MinInterval = d } }

// WithMaxRetries sets how many times a transient failure is retried. Negative
// values are clamped to 0 so at least one attempt is always made.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.MaxRetries = n
	}
}

// WithVerbose enables request logging to stderr.
func WithVerbose(v bool) Option { return func(c *Client) { c.Verbose = v } }

// New builds a Client with sensible defaults.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		APIKey:     apiKey,
		BaseURL:    DefaultBaseURL,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		UserAgent:  "nyt-cli (+https://github.com/derter/nyt)",
		MaxRetries: 3,
	}
	for _, o := range opts {
		o(c)
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	return c
}

// GetRaw performs an authenticated GET and returns the raw response body.
//
// path is appended to BaseURL (e.g. "/svc/topstories/v2/home.json"). The
// api-key parameter is added automatically; callers must not include it in q.
func (c *Client) GetRaw(ctx context.Context, path string, q url.Values) ([]byte, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("missing NYT API key")
	}
	if q == nil {
		q = url.Values{}
	}
	q.Set("api-key", c.APIKey)

	full := strings.TrimRight(c.BaseURL, "/") + path
	if enc := q.Encode(); enc != "" {
		full += "?" + enc
	}
	return c.rawGet(ctx, full, map[string]string{"Accept": "application/json"})
}

// GetExternal performs an unauthenticated GET against an absolute URL, reusing
// the same retry/backoff/throttle machinery as GetRaw. It is used for NYT's
// public RSS feeds (rss.nytimes.com), which carry no api-key and return XML.
func (c *Client) GetExternal(ctx context.Context, rawURL string) ([]byte, error) {
	return c.rawGet(ctx, rawURL, map[string]string{"Accept": "application/xml, text/xml"})
}

// GetHTML fetches an absolute URL sending caller-supplied headers (e.g. Cookie +
// a Chrome User-Agent), reusing the retry/throttle machinery. It is used by
// `nyt read` to pull full article HTML from nytimes.com past DataDome. A
// User-Agent in headers overrides the client's default; any other header is sent
// verbatim.
func (c *Client) GetHTML(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	return c.rawGet(ctx, rawURL, headers)
}

// rawGet runs the retry loop against an already-assembled URL, sending the given
// request headers.
func (c *Client) rawGet(ctx context.Context, full string, headers map[string]string) ([]byte, error) {
	var lastErr error
	// pendingWait carries a server-supplied Retry-After into the next iteration's
	// single sleep, so a 429 waits max(Retry-After, backoff) — never both added.
	var pendingWait time.Duration
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := backoff(attempt)
			if pendingWait > wait {
				wait = pendingWait
			}
			c.logf("retry %d after %s (%v)", attempt, wait, lastErr)
			if err := sleep(ctx, wait); err != nil {
				return nil, err
			}
		}
		c.throttle()

		body, retryAfter, err := c.do(ctx, full, headers)
		if err == nil {
			return body, nil
		}
		lastErr = err

		apiErr, ok := err.(*APIError)
		if !ok || !apiErr.Retryable() {
			return nil, err
		}
		pendingWait = retryAfter
	}
	if lastErr == nil {
		// Defensive: should be unreachable since MaxRetries is clamped to >= 0.
		lastErr = fmt.Errorf("request not attempted (retries=%d)", c.MaxRetries)
	}
	return nil, lastErr
}

// Get performs a GET and decodes the JSON response into out.
func (c *Client) Get(ctx context.Context, path string, q url.Values, out any) error {
	body, err := c.GetRaw(ctx, path, q)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}

// do issues a single request and returns the body, a Retry-After duration (if
// the server sent one), and an error. The redacted URL hides the API key in logs.
func (c *Client) do(ctx context.Context, full string, headers map[string]string) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// Default the User-Agent to the client's unless the caller supplied one.
	if _, ok := headers["User-Agent"]; !ok && c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	c.logf("GET %s", redact(full))

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// Network/transport errors are retryable.
		return nil, 0, &APIError{StatusCode: 0, Message: err.Error(), URL: redact(full)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, 0, &APIError{StatusCode: resp.StatusCode, Message: "reading body: " + err.Error(), URL: redact(full)}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, 0, nil
	}

	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	return nil, retryAfter, parseAPIError(resp.StatusCode, resp.Status, body, redact(full))
}

func (c *Client) throttle() {
	if c.MinInterval <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.lastCall.IsZero() {
		elapsed := time.Since(c.lastCall)
		if elapsed < c.MinInterval {
			time.Sleep(c.MinInterval - elapsed)
		}
	}
	c.lastCall = time.Now()
}

func (c *Client) logf(format string, args ...any) {
	if c.Verbose {
		fmt.Fprintf(stderr, "nyt: "+format+"\n", args...)
	}
}

func backoff(attempt int) time.Duration {
	// 0.5s, 1s, 2s, 4s ... capped at 16s.
	d := time.Duration(math.Pow(2, float64(attempt-1))) * 500 * time.Millisecond
	if d > 16*time.Second {
		d = 16 * time.Second
	}
	return d
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// redact removes the api-key value from a URL for logging.
func redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Get("api-key") != "" {
		q.Set("api-key", "REDACTED")
		u.RawQuery = q.Encode()
	}
	return u.String()
}
