package cmd

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/jo-nike/nyt_cli/internal/output"
	"github.com/spf13/cobra"
)

// articleSearchSorts are the valid values for --sort.
var articleSearchSorts = []string{"newest", "oldest", "relevance"}

// articleSearchResponse models /svc/search/v2/articlesearch.json.
type articleSearchResponse struct {
	Status   string                  `json:"status"`
	Response articleSearchResultData `json:"response"`
}

type articleSearchResultData struct {
	Meta   articleSearchMeta  `json:"meta"`
	Docs   []articleSearchDoc `json:"docs"`
	Facets map[string]any     `json:"facets"`
}

type articleSearchMeta struct {
	Hits   int `json:"hits"`
	Offset int `json:"offset"`
}

type articleSearchDoc struct {
	WebURL         string                `json:"web_url"`
	Snippet        string                `json:"snippet"`
	Abstract       string                `json:"abstract"`
	LeadParagraph  string                `json:"lead_paragraph"`
	PubDate        string                `json:"pub_date"`
	DocumentType   string                `json:"document_type"`
	TypeOfMaterial string                `json:"type_of_material"`
	SectionName    string                `json:"section_name"`
	SubsectionName string                `json:"subsection_name"`
	WordCount      int                   `json:"word_count"`
	Headline       articleSearchHeadline `json:"headline"`
	Byline         articleSearchByline   `json:"byline"`
}

type articleSearchHeadline struct {
	Main          string `json:"main"`
	Kicker        string `json:"kicker"`
	PrintHeadline string `json:"print_headline"`
}

type articleSearchByline struct {
	Original string `json:"original"`
}

func newArticleSearchCmd() *cobra.Command {
	var (
		fq          string
		begin       string
		end         string
		sortBy      string
		page        int
		fl          string
		facets      []string
		facetFilter bool
		hl          bool
	)

	cmd := &cobra.Command{
		Use:   "articlesearch [query]",
		Short: "Search NYTimes articles back to 1851 (Article Search API)",
		Long: `Search New York Times articles by keyword and Lucene filter query.

The positional [query] sets q. A search with no query is allowed when --fq is
given. Results are paginated 10 per page (--page 0..100).

Dates accept either YYYY-MM-DD or YYYYMMDD and are normalized to YYYYMMDD.
--sort must be one of: newest, oldest, relevance.`,
		Example: `  nyt articlesearch "climate change"
  nyt articlesearch "election" --begin 2024-01-01 --end 2024-11-05 --sort newest
  nyt articlesearch --fq 'section_name:("Sports")' --page 1
  nyt articlesearch "ai" --fl web_url,headline,pub_date --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := ""
			if len(args) == 1 {
				q = strings.TrimSpace(args[0])
			}
			if q == "" && fq == "" {
				return fmt.Errorf("provide a search [query] or --fq filter")
			}

			sortBy = strings.ToLower(strings.TrimSpace(sortBy))
			if !articleSearchValidSort(sortBy) {
				return fmt.Errorf("invalid --sort %q: must be one of %s", sortBy, strings.Join(articleSearchSorts, ", "))
			}
			if page < 0 || page > 100 {
				return fmt.Errorf("invalid --page %d: must be between 0 and 100", page)
			}

			query := url.Values{}
			addStr(query, "q", q)
			addStr(query, "fq", fq)
			addStr(query, "begin_date", articleSearchNormalizeDate(begin))
			addStr(query, "end_date", articleSearchNormalizeDate(end))
			addStr(query, "sort", sortBy)
			addInt(query, "page", page)
			addStr(query, "fl", fl)
			addStr(query, "facet_field", articleSearchJoinFacets(facets))
			addBool(query, "facet_filter", facetFilter)
			addBool(query, "hl", hl)

			c, err := apiClient()
			if err != nil {
				return err
			}
			path := "/svc/search/v2/articlesearch.json"

			raw, err := c.GetRaw(ctxOf(cmd), path, query)
			if err != nil {
				return err
			}
			if jsonOutput() {
				return output.PrettyJSON(cmd.OutOrStdout(), raw)
			}

			var resp articleSearchResponse
			if err := unmarshal(raw, &resp); err != nil {
				return err
			}
			renderArticleSearch(cmd.OutOrStdout(), &resp, q, page)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&fq, "fq", "", "Lucene filter query (e.g. 'section_name:(\"Sports\")')")
	f.StringVar(&begin, "begin", "", "earliest publication date (YYYY-MM-DD or YYYYMMDD)")
	f.StringVar(&end, "end", "", "latest publication date (YYYY-MM-DD or YYYYMMDD)")
	f.StringVar(&sortBy, "sort", "relevance", "sort order: newest, oldest, relevance")
	f.IntVar(&page, "page", 0, "result page, 0..100 (10 results per page)")
	f.StringVar(&fl, "fl", "", "comma-separated list of fields to return")
	f.StringSliceVar(&facets, "facet", nil, "facet field to compute (repeatable)")
	f.BoolVar(&facetFilter, "facet-filter", false, "apply fq when computing facet counts")
	f.BoolVar(&hl, "hl", false, "highlight search terms in snippet/headline")
	return cmd
}

// articleSearchValidSort reports whether s is an allowed --sort value.
func articleSearchValidSort(s string) bool {
	for _, v := range articleSearchSorts {
		if s == v {
			return true
		}
	}
	return false
}

// articleSearchNormalizeDate strips dashes so YYYY-MM-DD becomes YYYYMMDD.
// Empty input returns empty so the param is omitted.
func articleSearchNormalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(s, "-", "")
}

// articleSearchJoinFacets comma-joins repeated --facet values into facet_field.
func articleSearchJoinFacets(facets []string) string {
	cleaned := make([]string, 0, len(facets))
	for _, f := range facets {
		if f = strings.TrimSpace(f); f != "" {
			cleaned = append(cleaned, f)
		}
	}
	return strings.Join(cleaned, ",")
}

func renderArticleSearch(w io.Writer, resp *articleSearchResponse, q string, page int) {
	output.Header(w, fmt.Sprintf("Article Search — q=%q (%d hits, page %d)", q, resp.Response.Meta.Hits, page))
	if len(resp.Response.Docs) == 0 {
		fmt.Fprintln(w, "No articles found.")
		return
	}
	tw := output.NewTable(w)
	fmt.Fprintln(tw, "#\tDATE\tSECTION\tHEADLINE\tBYLINE")
	for i, d := range resp.Response.Docs {
		date := d.PubDate
		if len(date) > 10 {
			date = date[:10]
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1,
			output.Dash(date),
			output.Dash(output.Truncate(d.SectionName, 14)),
			output.Dash(output.Truncate(d.Headline.Main, 70)),
			output.Dash(output.Truncate(strings.TrimPrefix(d.Byline.Original, "By "), 24)),
		)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d results shown (10 per page, up to page 100). %d total hits. Use --json for full details.\n",
		len(resp.Response.Docs), resp.Response.Meta.Hits)
}

func init() {
	rootCmd.AddCommand(newArticleSearchCmd())
}
