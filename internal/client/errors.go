package client

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// stderr is the destination for verbose request logs (overridable in tests).
var stderr io.Writer = os.Stderr

// APIError describes a non-2xx response (or a transport failure when
// StatusCode == 0) from an NYT API.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
	Body       string
	URL        string
}

func (e *APIError) Error() string {
	switch {
	case e.StatusCode == 0:
		return fmt.Sprintf("request failed: %s", e.Message)
	case e.Message != "":
		return fmt.Sprintf("NYT API error %d: %s", e.StatusCode, e.Message)
	default:
		return fmt.Sprintf("NYT API error %d (%s)", e.StatusCode, e.Status)
	}
}

// Retryable reports whether the request may succeed if retried: transport
// errors (0), rate limiting (429), and server errors (5xx).
func (e *APIError) Retryable() bool {
	return e.StatusCode == 0 || e.StatusCode == 429 || e.StatusCode >= 500
}

// parseAPIError extracts a human-readable message from the several error
// envelopes the NYT APIs use (Apigee "fault", REST "errors", "message").
func parseAPIError(status int, statusText string, body []byte, redactedURL string) *APIError {
	e := &APIError{
		StatusCode: status,
		Status:     statusText,
		Body:       string(body),
		URL:        redactedURL,
	}

	var env struct {
		Fault struct {
			FaultString string `json:"faultstring"`
		} `json:"fault"`
		Message string          `json:"message"`
		Errors  json.RawMessage `json:"errors"`
		Error   string          `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil {
		switch {
		case env.Fault.FaultString != "":
			e.Message = env.Fault.FaultString
		case env.Message != "":
			e.Message = env.Message
		case env.Error != "":
			e.Message = env.Error
		case len(env.Errors) > 0:
			e.Message = joinErrors(env.Errors)
		}
	}

	// The Apigee gateway returns this fault when the key's app hasn't enabled
	// the requested API. Make the fix actionable.
	if strings.Contains(strings.ToLower(e.Message), "invalid apikey for given resource") {
		e.Message += " — enable this API for your app at https://developer.nytimes.com/my-apps"
	}

	if e.Message == "" {
		switch status {
		case 401:
			e.Message = "unauthorized — check your API key and that this API is enabled for your app at https://developer.nytimes.com/my-apps"
		case 403:
			e.Message = "forbidden — your key's app may not have this API enabled (https://developer.nytimes.com/my-apps)"
		case 404:
			e.Message = "not found — check the path/section/list parameters, or enable this API for your app at https://developer.nytimes.com/my-apps"
		case 429:
			e.Message = "rate limit exceeded — slow down (try --throttle 6s) or wait for your daily quota to reset"
		}
	}
	return e
}

// joinErrors renders the "errors" field, which may be a []string or a [{...}].
func joinErrors(raw json.RawMessage) string {
	var strs []string
	if json.Unmarshal(raw, &strs) == nil && len(strs) > 0 {
		return strings.Join(strs, "; ")
	}
	var objs []map[string]any
	if json.Unmarshal(raw, &objs) == nil && len(objs) > 0 {
		var parts []string
		for _, o := range objs {
			if m, ok := o["message"].(string); ok {
				parts = append(parts, m)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	return strings.TrimSpace(string(raw))
}
