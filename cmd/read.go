package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jo-nike/nyt_cli/internal/client"
	"github.com/jo-nike/nyt_cli/internal/config"
	"github.com/jo-nike/nyt_cli/internal/output"
	readability "github.com/go-shiori/go-readability"
	"github.com/spf13/cobra"
	"golang.org/x/net/html"
)

// blockTags are HTML block-level elements; a newline is inserted around each so
// paragraphs don't run together when we flatten the cleaned article to text.
var blockTags = map[string]bool{
	"p": true, "div": true, "section": true, "article": true, "header": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "blockquote": true, "br": true, "tr": true,
}

// skipTags are subtrees dropped wholesale: media figures (image + caption +
// credit), scripts/styles, and non-article asides — none of which is body text.
var skipTags = map[string]bool{
	"figure": true, "figcaption": true, "script": true, "style": true,
	"noscript": true, "svg": true, "aside": true,
}

// chromeUA is a real Chrome User-Agent. DataDome flags the default API UA (and
// curl's TLS fingerprint), but lets Go's stdlib client through with this UA.
const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// readResult is the lean, LLM-friendly --json shape. The raw article HTML is
// deliberately omitted to keep token cost down.
type readResult struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Published string `json:"published,omitempty"`
	Excerpt   string `json:"excerpt,omitempty"`
	Text      string `json:"text"`
}

func newReadCmd() *cobra.Command {
	var flagCookie string

	cmd := &cobra.Command{
		Use:   "read <url>",
		Short: "Fetch and extract the full text of a NYTimes.com article",
		Long: `Fetch a nytimes.com article URL and print its readable full text.

The NYT APIs only return metadata, so the body is pulled directly from
nytimes.com. This requires your browser cookie (subscription + DataDome),
supplied (in priority order) via:
    --cookie flag, $NYT_COOKIE, a .env file, or ~/.config/nyt/config.json.

Copy the cookie from DevTools → Application → Cookies (it must include the
NYT-S and datadome values). No NYT API key is needed for this command.

Cookies rotate: when yours expires the fetch is blocked and you'll be told to
refresh it.`,
		Example: `  nyt read "https://www.nytimes.com/2026/06/22/world/example.html"
  nyt read "<url>" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := url.Parse(strings.TrimSpace(args[0]))
			if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return fmt.Errorf("invalid article URL %q — pass an absolute http(s) nytimes.com URL", args[0])
			}

			cookie, source, err := config.ResolveCookie(flagCookie)
			if err != nil {
				return err
			}
			if flagVerbose {
				fmt.Fprintf(os.Stderr, "nyt: using cookie from %s\n", source)
			}

			c, err := rssClient()
			if err != nil {
				return err
			}
			headers := map[string]string{
				"User-Agent":      chromeUA,
				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
				"Accept-Language": "en-US,en;q=0.5",
				"Cookie":          cookie,
			}
			raw, err := c.GetHTML(ctxOf(cmd), parsed.String(), headers)
			if err != nil {
				return readFetchError(err)
			}

			art, err := readability.FromReader(bytes.NewReader(raw), parsed)
			if err != nil {
				return fmt.Errorf("could not extract article text: %w", err)
			}

			text := art.TextContent
			if art.Node != nil {
				text = articleText(art.Node)
			}

			if jsonOutput() {
				res := readResult{
					URL:     parsed.String(),
					Title:   art.Title,
					Author:  art.Byline,
					Excerpt: art.Excerpt,
					Text:    text,
				}
				if art.PublishedTime != nil {
					res.Published = art.PublishedTime.Format(time.RFC3339)
				}
				return output.JSON(cmd.OutOrStdout(), res)
			}

			out := cmd.OutOrStdout()
			if art.Title != "" {
				output.Header(out, art.Title)
			}
			if art.Byline != "" {
				fmt.Fprintln(out, art.Byline)
			}
			if art.Title != "" || art.Byline != "" {
				fmt.Fprintln(out)
			}
			fmt.Fprintln(out, text)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagCookie, "cookie", "", "NYT browser cookie (overrides env/config)")
	return cmd
}

// articleText flattens go-readability's cleaned content node into readable,
// paragraph-separated text. Readability's TextContent runs every paragraph
// together and still includes image captions/credits, ad slots, and the
// "Related Content" recirculation block; this walk drops those and inserts
// blank lines between block elements.
func articleText(root *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			if skipTags[n.Data] || nodeAttr(n, "data-testid") == "recirculation-placeholder" {
				return
			}
			block := blockTags[n.Data]
			if block {
				b.WriteByte('\n')
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if block {
				b.WriteByte('\n')
			}
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(root)

	var paras []string
	for _, line := range strings.Split(b.String(), "\n") {
		line = strings.Join(strings.Fields(line), " ") // collapse whitespace, trim
		if line == "" || isBoilerplate(line) {
			continue
		}
		paras = append(paras, line)
	}
	return strings.Join(paras, "\n\n")
}

// isBoilerplate reports whether a flattened line is ad chrome left in the body.
func isBoilerplate(line string) bool {
	switch strings.ToLower(line) {
	case "advertisement", "skip advertisement":
		return true
	}
	return false
}

// nodeAttr returns the value of the named attribute, or "".
func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// readFetchError turns a DataDome 403 into an actionable message instead of the
// raw API-key-flavored 403 or a dump of the captcha HTML.
func readFetchError(err error) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 403 {
		return fmt.Errorf("NYT cookie expired or invalid — refresh NYT_COOKIE from DevTools → Application → Cookies")
	}
	return err
}

func init() {
	rootCmd.AddCommand(newReadCmd())
}
