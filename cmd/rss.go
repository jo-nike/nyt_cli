package cmd

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/derter/nyt/internal/output"
	"github.com/spf13/cobra"
)

// rssFeedBase is the host + path prefix for NYT's public RSS feeds. The feed
// name and ".xml" suffix are appended per request. It is a package var so tests
// can point it at an httptest server.
var rssFeedBase = "https://rss.nytimes.com/services/xml/rss/nyt"

// rssSections are the NYT RSS feeds confirmed to be served. The positional
// section argument accepts any name; unknown feeds surface NYT's own 404.
var rssSections = []string{
	"HomePage", "World", "US", "Business", "Technology", "Science", "Health",
	"Sports", "Arts", "Books", "Movies", "Travel", "Opinion", "Education", "Magazine",
}

// ---------------------------------------------------------------------------
// RSS 2.0 XML model. dc:creator matches by local name "creator".
// ---------------------------------------------------------------------------

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string     `xml:"title"`
	AtomLink      []struct{} `xml:"http://www.w3.org/2005/Atom link"` // absorb <atom:link> so it can't clobber <link>
	Link          string     `xml:"link"`
	Description   string     `xml:"description"`
	LastBuildDate string     `xml:"lastBuildDate"`
	Items         []rssItem  `xml:"item"`
}

type rssItem struct {
	Title       string     `xml:"title"`
	AtomLink    []struct{} `xml:"http://www.w3.org/2005/Atom link"` // absorb <atom:link> so it can't clobber <link>
	Link        string     `xml:"link"`
	GUID        string     `xml:"guid"`
	Description string     `xml:"description"`
	Creator     string     `xml:"creator"`
	PubDate     string     `xml:"pubDate"`
	Category    []string   `xml:"category"`
}

// ---------------------------------------------------------------------------
// Normalized output. Marshaled directly for --json so RSS output mirrors the
// shape of the other commands (clean JSON) rather than leaking raw XML.
// ---------------------------------------------------------------------------

type rssResult struct {
	Section  string       `json:"section"`
	Link     string       `json:"link"`
	Updated  string       `json:"updated"`
	NumItems int          `json:"num_items"`
	Items    []rssOutItem `json:"items"`
}

type rssOutItem struct {
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	Byline     string   `json:"byline"`
	Published  string   `json:"published"`
	Abstract   string   `json:"abstract"`
	Categories []string `json:"categories"`
}

func newRSSCmd() *cobra.Command {
	var (
		limit       int
		sectionList bool
	)

	cmd := &cobra.Command{
		Use:   "rss [section]",
		Short: "Latest articles by section from NYT's public RSS feeds",
		Long: `Read the New York Times public RSS feeds — a real-time feed of the latest
articles in a section. These feeds are free and need no API key.

section defaults to "HomePage". Run "nyt rss --sections" to list the known
feeds; any feed name NYT publishes is accepted.

Note: unlike other commands, --json here is a normalized representation of the
feed (NYT serves RSS as XML, not JSON), not a raw upstream passthrough.`,
		Example: `  nyt rss
  nyt rss technology
  nyt rss world --limit 10
  nyt rss business --json
  nyt rss --sections`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if sectionList {
				renderRSSSections(cmd.OutOrStdout())
				return nil
			}
			if limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", limit)
			}

			section := "HomePage"
			if len(args) >= 1 && strings.TrimSpace(args[0]) != "" {
				section = strings.TrimSpace(args[0])
			}

			c, err := rssClient()
			if err != nil {
				return err
			}
			feedURL := fmt.Sprintf("%s/%s.xml", strings.TrimRight(rssFeedBase, "/"), url.PathEscape(section))
			raw, err := c.GetExternal(ctxOf(cmd), feedURL)
			if err != nil {
				return err
			}

			feed, err := parseRSS(raw)
			if err != nil {
				return err
			}
			result := toRSSResult(feed, limit)
			if jsonOutput() {
				return output.JSON(cmd.OutOrStdout(), result)
			}
			renderRSS(cmd.OutOrStdout(), section, result)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of items to show")
	cmd.Flags().BoolVar(&sectionList, "sections", false, "list the known RSS feed names and exit")
	return cmd
}

// parseRSS decodes RSS 2.0 XML into the feed model.
func parseRSS(raw []byte) (*rssFeed, error) {
	var feed rssFeed
	if err := xml.Unmarshal(raw, &feed); err != nil {
		return nil, fmt.Errorf("parsing RSS feed: %w", err)
	}
	return &feed, nil
}

// toRSSResult normalizes a parsed feed into the output shape, capping items at
// limit (RSS feeds have no server-side limit parameter).
func toRSSResult(feed *rssFeed, limit int) rssResult {
	items := feed.Channel.Items
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := rssResult{
		Section:  feed.Channel.Title,
		Link:     feed.Channel.Link,
		Updated:  feed.Channel.LastBuildDate,
		NumItems: len(items),
		Items:    make([]rssOutItem, 0, len(items)),
	}
	for _, it := range items {
		out.Items = append(out.Items, rssOutItem{
			Title:      it.Title,
			URL:        it.Link,
			Byline:     strings.TrimSpace(strings.TrimPrefix(it.Creator, "By ")),
			Published:  it.PubDate,
			Abstract:   it.Description,
			Categories: it.Category,
		})
	}
	return out
}

func renderRSS(w io.Writer, section string, r rssResult) {
	title := r.Section
	if title == "" {
		title = section
	}
	output.Header(w, fmt.Sprintf("NYT RSS — %s", title))
	if len(r.Items) == 0 {
		fmt.Fprintln(w, "No items found.")
		return
	}
	tw := output.NewTable(w)
	fmt.Fprintln(tw, "#\tTITLE\tBYLINE\tPUBLISHED")
	for i, it := range r.Items {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
			i+1,
			output.Dash(output.Truncate(it.Title, 64)),
			output.Dash(output.Truncate(it.Byline, 22)),
			output.Dash(output.Truncate(it.Published, 16)),
		)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d items. Use --json for URLs, abstracts, and categories.\n", len(r.Items))
}

func renderRSSSections(w io.Writer) {
	output.Header(w, "NYT RSS feeds")
	tw := output.NewTable(w)
	for _, s := range rssSections {
		fmt.Fprintln(tw, s)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d feeds. Pass one as the section, e.g. \"nyt rss Technology\". Other NYT feed names also work.\n", len(rssSections))
}

func init() {
	rootCmd.AddCommand(newRSSCmd())
}
