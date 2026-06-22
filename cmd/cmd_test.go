package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// executeRoot runs the root command with args, capturing output. Used only for
// paths that fail before any network call (arg validation, unknown commands).
func executeRoot(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestRootSubcommandsRegistered(t *testing.T) {
	want := []string{
		"topstories", "archive", "articlesearch", "mostpopular",
		"books", "rss", "config", "version",
	}
	have := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		have[c.Name()] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("subcommand %q not registered on root", w)
		}
	}
}

func TestArchiveRejectsBadYear(t *testing.T) {
	if _, err := executeRoot("archive", "1700", "5"); err == nil {
		t.Fatal("expected an error for an out-of-range year")
	} else if !strings.Contains(err.Error(), "range") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnknownBooksSubcommandErrors(t *testing.T) {
	_, err := executeRoot("books", "bogus")
	if err == nil || !strings.Contains(err.Error(), "unknown books subcommand") {
		t.Fatalf("expected unknown-subcommand error, got %v", err)
	}
}

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>NYT > Technology</title>
    <link>https://www.nytimes.com/section/technology</link>
    <atom:link href="https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml" rel="self"></atom:link>
    <lastBuildDate>Mon, 22 Jun 2026 12:00:00 +0000</lastBuildDate>
    <item>
      <title>First Story</title>
      <link>https://www.nytimes.com/2026/06/22/technology/first.html</link>
      <description>An abstract.</description>
      <dc:creator>By Ada Lovelace</dc:creator>
      <pubDate>Mon, 22 Jun 2026 11:00:00 +0000</pubDate>
      <category domain="http://www.nytimes.com/namespaces/keywords/des">Computers</category>
      <category domain="http://www.nytimes.com/namespaces/keywords/nyt_org">NYT</category>
      <atom:link href="https://www.nytimes.com/2026/06/22/technology/first.html" rel="standout"></atom:link>
    </item>
    <item>
      <title>Second Story</title>
      <link>https://www.nytimes.com/2026/06/22/technology/second.html</link>
      <description>Another abstract.</description>
      <dc:creator>By Alan Turing</dc:creator>
      <pubDate>Mon, 22 Jun 2026 10:00:00 +0000</pubDate>
    </item>
    <item>
      <title>Third Story</title>
      <link>https://www.nytimes.com/2026/06/22/technology/third.html</link>
      <dc:creator>By Grace Hopper</dc:creator>
    </item>
  </channel>
</rss>`

func TestParseAndNormalizeRSS(t *testing.T) {
	feed, err := parseRSS([]byte(sampleRSS))
	if err != nil {
		t.Fatalf("parseRSS: %v", err)
	}
	if got := feed.Channel.Title; got != "NYT > Technology" {
		t.Errorf("channel title = %q", got)
	}
	if len(feed.Channel.Items) != 3 {
		t.Fatalf("parsed %d items, want 3", len(feed.Channel.Items))
	}

	// --limit caps the items.
	r := toRSSResult(feed, 2)
	if r.NumItems != 2 || len(r.Items) != 2 {
		t.Fatalf("limit not applied: NumItems=%d len=%d", r.NumItems, len(r.Items))
	}

	// <atom:link> must not clobber the plain channel <link>.
	if r.Link != "https://www.nytimes.com/section/technology" {
		t.Errorf("channel link = %q (atom:link likely clobbered it)", r.Link)
	}

	first := r.Items[0]
	if first.Byline != "Ada Lovelace" { // "By " prefix stripped
		t.Errorf("byline = %q, want %q", first.Byline, "Ada Lovelace")
	}
	if first.URL != "https://www.nytimes.com/2026/06/22/technology/first.html" {
		t.Errorf("url = %q", first.URL)
	}
	if first.Abstract != "An abstract." {
		t.Errorf("abstract = %q", first.Abstract)
	}
	if len(first.Categories) != 2 || first.Categories[0] != "Computers" {
		t.Errorf("categories = %v", first.Categories)
	}
}
