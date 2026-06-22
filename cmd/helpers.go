package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// unmarshal decodes JSON with a friendlier error message than the raw decoder.
func unmarshal(b []byte, v any) error {
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("could not parse NYT response: %w", err)
	}
	return nil
}

// addStr sets q[key]=val only when val is non-empty.
func addStr(q url.Values, key, val string) {
	if val != "" {
		q.Set(key, val)
	}
}

// addInt sets q[key]=val only when val is non-zero.
func addInt(q url.Values, key string, val int) {
	if val != 0 {
		q.Set(key, strconv.Itoa(val))
	}
}

// addBool sets q[key]="true" only when val is true.
func addBool(q url.Values, key string, val bool) {
	if val {
		q.Set(key, "true")
	}
}
