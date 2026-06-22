package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseAPIErrorEnvelopes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"apigee fault", `{"fault":{"faultstring":"Invalid ApiKey for given resource"}}`, "enable this API"},
		{"errors array of strings", `{"status":"ERROR","errors":["list not found"]}`, "list not found"},
		{"message field", `{"message":"bad request"}`, "bad request"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := parseAPIError(400, "400 Bad Request", []byte(c.body), "https://api.nytimes.com/x")
			if !strings.Contains(e.Error(), c.want) {
				t.Fatalf("got %q, want it to contain %q", e.Error(), c.want)
			}
		})
	}
}

func TestRetryableClassification(t *testing.T) {
	for status, want := range map[int]bool{0: true, 429: true, 500: true, 503: true, 400: false, 404: false, 200: false} {
		if got := (&APIError{StatusCode: status}).Retryable(); got != want {
			t.Errorf("status %d retryable=%v, want %v", status, got, want)
		}
	}
}

func TestRedactStripsAPIKey(t *testing.T) {
	got := redact("https://api.nytimes.com/svc/x.json?q=hi&api-key=SECRET123")
	if strings.Contains(got, "SECRET123") {
		t.Fatalf("api key leaked: %s", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("expected REDACTED, got %s", got)
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	if d := parseRetryAfter("12"); d != 12*time.Second {
		t.Fatalf("got %v, want 12s", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Fatalf("got %v, want 0", d)
	}
}

func TestGetRawInjectsKeyAndRetries(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api-key") != "K" {
			t.Errorf("api-key not injected: %s", r.URL.RawQuery)
		}
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	defer srv.Close()

	c := New("K", WithBaseURL(srv.URL), WithMaxRetries(2))
	body, err := c.GetRaw(context.Background(), "/svc/x.json", url.Values{"q": {"hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"status":"OK"}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (one retry), got %d", calls)
	}
}

func TestGetRawMissingKey(t *testing.T) {
	c := New("")
	if _, err := c.GetRaw(context.Background(), "/x", nil); err == nil {
		t.Fatal("expected error for missing key")
	}
}
