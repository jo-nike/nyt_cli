package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"hello world this is long", "hello worl…"},
	}
	for _, c := range cases {
		if got := Truncate(c.in, 11); got != c.want {
			t.Errorf("Truncate(%q,11) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := Truncate("multi\nline\ttext", 0); got != "multi line text" {
		t.Errorf("newline/tab not normalized: %q", got)
	}
}

func TestDash(t *testing.T) {
	if Dash("") != "-" || Dash("   ") != "-" {
		t.Error("empty should become -")
	}
	if Dash("x") != "x" {
		t.Error("non-empty should pass through")
	}
}

func TestPrettyJSON(t *testing.T) {
	var b bytes.Buffer
	if err := PrettyJSON(&b, []byte(`{"a":1,"b":[2,3]}`)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\n  \"a\": 1") {
		t.Fatalf("not indented: %s", b.String())
	}
}

func TestPrettyJSONInvalidPassThrough(t *testing.T) {
	var b bytes.Buffer
	if err := PrettyJSON(&b, []byte(`not json`)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "not json") {
		t.Fatalf("invalid json should pass through: %s", b.String())
	}
}
