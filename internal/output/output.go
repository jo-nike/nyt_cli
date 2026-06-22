// Package output renders command results either as the raw NYT JSON
// (re-indented) or as compact, human-readable tables.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
)

// PrettyJSON re-indents and writes raw JSON bytes. If the bytes are not valid
// JSON they are written through unchanged.
func PrettyJSON(w io.Writer, raw []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		_, werr := w.Write(raw)
		if werr == nil {
			_, werr = io.WriteString(w, "\n")
		}
		return werr
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// JSON marshals v as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// NewTable returns a tabwriter that aligns tab-separated columns. Callers write
// rows with '\t' separators and must call Flush when done.
func NewTable(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
}

// Truncate shortens s to max runes, appending an ellipsis. It collapses all
// internal whitespace (newlines, tabs, runs of spaces) to single spaces so a
// value never breaks the tab-separated table layout. A max <= 0 only normalizes.
func Truncate(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	if max <= 1 {
		return string([]rune(s)[:max])
	}
	return string([]rune(s)[:max-1]) + "…"
}

// Dash returns "-" for empty strings so table cells are never blank.
func Dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// Header prints a bold-ish section header followed by a rule.
func Header(w io.Writer, title string) {
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, strings.Repeat("─", utf8.RuneCountInString(title)))
}
