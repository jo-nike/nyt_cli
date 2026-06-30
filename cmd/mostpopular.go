package cmd

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/jo-nike/nyt_cli/internal/output"
	"github.com/spf13/cobra"
)

// mostPopularMetrics lists the valid metrics for the Most Popular API.
var mostPopularMetrics = []string{"viewed", "emailed", "shared"}

// mostPopularPeriods lists the valid time windows (in days).
var mostPopularPeriods = []string{"1", "7", "30"}

// mostPopularResponse models /svc/mostpopular/v2/{metric}/{period}.json.
type mostPopularResponse struct {
	Status     string                  `json:"status"`
	NumResults int                     `json:"num_results"`
	Results    []mostPopularResultItem `json:"results"`
}

type mostPopularResultItem struct {
	URL           string   `json:"url"`
	Section       string   `json:"section"`
	Subsection    string   `json:"subsection"`
	Byline        string   `json:"byline"`
	Title         string   `json:"title"`
	Abstract      string   `json:"abstract"`
	PublishedDate string   `json:"published_date"`
	Source        string   `json:"source"`
	Type          string   `json:"type"`
	DesFacet      []string `json:"des_facet"`
	OrgFacet      []string `json:"org_facet"`
	PerFacet      []string `json:"per_facet"`
	GeoFacet      []string `json:"geo_facet"`
	Media         []any    `json:"media"`
}

func newMostPopularCmd() *cobra.Command {
	var shareType string

	cmd := &cobra.Command{
		Use:   "mostpopular <metric> [period]",
		Short: "Most viewed, emailed, or shared NYTimes articles (Most Popular API)",
		Long: `Fetch the most popular NYTimes.com articles for a metric over a time window.

metric (required) is one of: viewed, emailed, shared.
period (optional) is the number of days: 1, 7, or 30 (default 7).

The --share-type flag is only valid with the "shared" metric and narrows the
results to a specific platform (facebook is the reliable value).`,
		Example: `  nyt mostpopular viewed
  nyt mostpopular emailed 1
  nyt mostpopular shared 30
  nyt mostpopular shared 7 --share-type facebook
  nyt mostpopular viewed 30 --json`,
		Args:      cobra.RangeArgs(1, 2),
		ValidArgs: mostPopularMetrics,
		RunE: func(cmd *cobra.Command, args []string) error {
			metric := strings.ToLower(strings.TrimSpace(args[0]))
			if !mostPopularContains(mostPopularMetrics, metric) {
				return fmt.Errorf("invalid metric %q: valid metrics are %s",
					args[0], strings.Join(mostPopularMetrics, ", "))
			}

			period := "7"
			if len(args) == 2 {
				period = strings.TrimSpace(args[1])
			}
			if !mostPopularContains(mostPopularPeriods, period) {
				return fmt.Errorf("invalid period %q: valid periods are %s (days)",
					period, strings.Join(mostPopularPeriods, ", "))
			}

			shareType = strings.ToLower(strings.TrimSpace(shareType))
			if shareType != "" && metric != "shared" {
				return fmt.Errorf("--share-type is only valid with the \"shared\" metric, not %q", metric)
			}

			var path string
			if metric == "shared" && shareType != "" {
				path = fmt.Sprintf("/svc/mostpopular/v2/shared/%s/%s.json",
					url.PathEscape(period), url.PathEscape(shareType))
			} else {
				path = fmt.Sprintf("/svc/mostpopular/v2/%s/%s.json",
					url.PathEscape(metric), url.PathEscape(period))
			}

			c, err := apiClient()
			if err != nil {
				return err
			}
			raw, err := c.GetRaw(ctxOf(cmd), path, nil)
			if err != nil {
				return err
			}
			if jsonOutput() {
				return output.PrettyJSON(cmd.OutOrStdout(), raw)
			}

			var resp mostPopularResponse
			if err := unmarshal(raw, &resp); err != nil {
				return err
			}
			renderMostPopular(cmd.OutOrStdout(), &resp, metric, period)
			return nil
		},
	}

	cmd.Flags().StringVar(&shareType, "share-type", "", "platform filter for the shared metric (e.g. facebook)")
	return cmd
}

func renderMostPopular(w io.Writer, resp *mostPopularResponse, metric, period string) {
	output.Header(w, fmt.Sprintf("Most %s — last %s day(s)", metric, period))
	if len(resp.Results) == 0 {
		fmt.Fprintln(w, "No articles found.")
		return
	}
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
	fmt.Fprintf(w, "\n%d articles. Use --json for full details (URLs, abstracts, facets, media).\n", len(resp.Results))
}

func mostPopularContains(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(newMostPopularCmd())
}
