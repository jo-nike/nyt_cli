package cmd

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"gitea.jonn.me/jons-org/nyt_cli/internal/output"
	"github.com/spf13/cobra"
)

// knownTopStoriesSections is used for --sections and shell completion. The API
// may add sections over time, so it is NOT used to reject input.
var knownTopStoriesSections = []string{
	"arts", "automobiles", "books", "business", "fashion", "food", "health",
	"home", "insider", "magazine", "movies", "nyregion", "obituaries",
	"opinion", "politics", "realestate", "science", "sports", "sundayreview",
	"technology", "theater", "t-magazine", "travel", "upshot", "us", "world",
}

// topStoriesResponse models /svc/topstories/v2/{section}.json.
type topStoriesResponse struct {
	Status      string         `json:"status"`
	Section     string         `json:"section"`
	LastUpdated string         `json:"last_updated"`
	NumResults  int            `json:"num_results"`
	Results     []topStoryItem `json:"results"`
}

type topStoryItem struct {
	Section       string `json:"section"`
	Subsection    string `json:"subsection"`
	Title         string `json:"title"`
	Abstract      string `json:"abstract"`
	URL           string `json:"url"`
	Byline        string `json:"byline"`
	ItemType      string `json:"item_type"`
	UpdatedDate   string `json:"updated_date"`
	PublishedDate string `json:"published_date"`
}

func newTopStoriesCmd() *cobra.Command {
	var listSections bool

	cmd := &cobra.Command{
		Use:   "topstories [section]",
		Short: "Articles currently on a NYTimes.com section front (Top Stories API)",
		Long: `Fetch the articles currently on a NYTimes.com section front.

The section defaults to "home". Run with --sections to list the valid sections.`,
		Example: `  nyt topstories
  nyt topstories technology
  nyt topstories world --json
  nyt topstories --sections`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: knownTopStoriesSections,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listSections {
				out := cmd.OutOrStdout()
				output.Header(out, "Top Stories sections")
				sorted := append([]string(nil), knownTopStoriesSections...)
				sort.Strings(sorted)
				for _, s := range sorted {
					fmt.Fprintln(out, "  "+s)
				}
				return nil
			}

			section := "home"
			if len(args) == 1 {
				section = strings.ToLower(strings.TrimSpace(args[0]))
			}

			c, err := apiClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/svc/topstories/v2/%s.json", url.PathEscape(section))

			raw, err := c.GetRaw(ctxOf(cmd), path, nil)
			if err != nil {
				return err
			}
			if jsonOutput() {
				return output.PrettyJSON(cmd.OutOrStdout(), raw)
			}

			var resp topStoriesResponse
			if err := unmarshal(raw, &resp); err != nil {
				return err
			}
			renderTopStories(cmd.OutOrStdout(), &resp)
			return nil
		},
	}

	cmd.Flags().BoolVar(&listSections, "sections", false, "list valid section names and exit")
	return cmd
}

func renderTopStories(w io.Writer, resp *topStoriesResponse) {
	if len(resp.Results) == 0 {
		fmt.Fprintln(w, "No stories found.")
		return
	}
	output.Header(w, fmt.Sprintf("Top Stories — %s (updated %s)", resp.Section, resp.LastUpdated))
	tw := output.NewTable(w)
	fmt.Fprintln(tw, "#\tSECTION\tTITLE\tBYLINE")
	for i, it := range resp.Results {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
			i+1,
			output.Dash(output.Truncate(it.Section, 14)),
			output.Dash(output.Truncate(it.Title, 70)),
			output.Dash(output.Truncate(strings.TrimPrefix(it.Byline, "By "), 24)),
		)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d stories. Use --json for full details (URLs, abstracts, multimedia).\n", len(resp.Results))
}

func init() {
	rootCmd.AddCommand(newTopStoriesCmd())
}
