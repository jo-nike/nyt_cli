package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/jo-nike/nyt_cli/internal/output"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Response types for the Books API (/svc/books/v3).
// ---------------------------------------------------------------------------

// booksListsResponse models /svc/books/v3/lists/names.json.
type booksListsResponse struct {
	Status     string                `json:"status"`
	NumResults int                   `json:"num_results"`
	Results    []booksListNameResult `json:"results"`
}

type booksListNameResult struct {
	ListName            string `json:"list_name"`
	DisplayName         string `json:"display_name"`
	ListNameEncoded     string `json:"list_name_encoded"`
	OldestPublishedDate string `json:"oldest_published_date"`
	NewestPublishedDate string `json:"newest_published_date"`
	Updated             string `json:"updated"`
}

// booksListResponse models a current/dated best-seller list. The API returns
// "results" as a single object whose "books" array holds the ranked titles
// (NOT an array of book_details, which was the shape of the retired
// /svc/books/v3/lists.json?list= endpoint).
type booksListResponse struct {
	Status     string          `json:"status"`
	NumResults int             `json:"num_results"`
	Results    booksListResult `json:"results"`
}

type booksListResult struct {
	ListName        string          `json:"list_name"`
	DisplayName     string          `json:"display_name"`
	ListNameEncoded string          `json:"list_name_encoded"`
	BestsellersDate string          `json:"bestsellers_date"`
	PublishedDate   string          `json:"published_date"`
	Updated         string          `json:"updated"`
	Books           []booksListBook `json:"books"`
}

type booksListBook struct {
	Rank          int    `json:"rank"`
	RankLastWeek  int    `json:"rank_last_week"`
	WeeksOnList   int    `json:"weeks_on_list"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	Publisher     string `json:"publisher"`
	PrimaryISBN13 string `json:"primary_isbn13"`
}

// booksListInfo is a normalized catalog row used by "books lists", populated
// from either lists/names.json or the overview fallback.
type booksListInfo struct {
	encoded string
	display string
	cadence string
	newest  string
}

// booksOverviewResponse models /svc/books/v3/lists/overview.json.
type booksOverviewResponse struct {
	Status  string              `json:"status"`
	Results booksOverviewResult `json:"results"`
}

type booksOverviewResult struct {
	BestsellersDate string              `json:"bestsellers_date"`
	PublishedDate   string              `json:"published_date"`
	Lists           []booksOverviewList `json:"lists"`
}

type booksOverviewList struct {
	DisplayName     string              `json:"display_name"`
	ListNameEncoded string              `json:"list_name_encoded"`
	Updated         string              `json:"updated"`
	Books           []booksOverviewBook `json:"books"`
}

type booksOverviewBook struct {
	Rank   int    `json:"rank"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

// ---------------------------------------------------------------------------
// Command tree.
// ---------------------------------------------------------------------------

func newBooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "books",
		Short: "NYT best-seller lists (Books API)",
		Long: `Query the New York Times Books API: best-seller lists and list overviews.

All data comes from /svc/books/v3. Start with "nyt books lists" to discover the
encoded list names used by "nyt books list".`,
		Example: `  nyt books lists
  nyt books list hardcover-fiction
  nyt books list combined-print-and-e-book-fiction --date 2024-01-07
  nyt books overview --date 2024-01-07`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown books subcommand %q", args[0])
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newBooksListsCmd(),
		newBooksListCmd(),
		newBooksOverviewCmd(),
	)
	return cmd
}

// ---------------------------------------------------------------------------
// books lists
// ---------------------------------------------------------------------------

func newBooksListsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lists",
		Short: "Available best-seller list names",
		Long: `List the names of all available New York Times best-seller lists.

Primary source is /svc/books/v3/lists/names.json. If that endpoint is not
provisioned for your app, the catalog is derived from the overview endpoint,
which also exposes each list's encoded name.`,
		Example: `  nyt books lists
  nyt books lists --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := apiClient()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			// Primary: lists/names.json. Honor it only when it actually returns
			// lists, so a 200-with-empty-results (the upstream-limited case) falls
			// through to the overview fallback in BOTH table and --json modes.
			raw, nerr := c.GetRaw(ctxOf(cmd), "/svc/books/v3/lists/names.json", nil)
			if nerr == nil {
				var resp booksListsResponse
				if unmarshal(raw, &resp) == nil && len(resp.Results) > 0 {
					if jsonOutput() {
						return output.PrettyJSON(out, raw)
					}
					infos := make([]booksListInfo, 0, len(resp.Results))
					for _, it := range resp.Results {
						infos = append(infos, booksListInfo{it.ListNameEncoded, it.DisplayName, it.Updated, it.NewestPublishedDate})
					}
					renderBooksLists(out, infos)
					return nil
				}
			}

			// Fallback: derive from the overview endpoint.
			raw2, oerr := c.GetRaw(ctxOf(cmd), "/svc/books/v3/lists/overview.json", nil)
			if oerr != nil {
				if nerr != nil {
					return fmt.Errorf("lists/names.json unavailable (%v); overview fallback also failed: %w", nerr, oerr)
				}
				return oerr
			}
			if jsonOutput() {
				return output.PrettyJSON(out, raw2)
			}
			var ov booksOverviewResponse
			if err := unmarshal(raw2, &ov); err != nil {
				return err
			}
			infos := make([]booksListInfo, 0, len(ov.Results.Lists))
			for _, l := range ov.Results.Lists {
				infos = append(infos, booksListInfo{l.ListNameEncoded, l.DisplayName, l.Updated, ov.Results.PublishedDate})
			}
			fmt.Fprintln(os.Stderr, "note: lists/names.json is not available for this key; catalog derived from overview.json")
			renderBooksLists(out, infos)
			return nil
		},
	}
}

func renderBooksLists(w io.Writer, infos []booksListInfo) {
	if len(infos) == 0 {
		fmt.Fprintln(w, "No lists found.")
		return
	}
	output.Header(w, "Best-seller lists")
	tw := output.NewTable(w)
	fmt.Fprintln(tw, "ENCODED\tDISPLAY\tCADENCE\tNEWEST")
	for _, it := range infos {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			output.Dash(output.Truncate(it.encoded, 40)),
			output.Dash(output.Truncate(it.display, 40)),
			output.Dash(it.cadence),
			output.Dash(it.newest),
		)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d lists. Use the ENCODED name with \"nyt books list <name>\".\n", len(infos))
}

// ---------------------------------------------------------------------------
// books list <list-name>
// ---------------------------------------------------------------------------

func newBooksListCmd() *cobra.Command {
	var date, sortBy, sortOrder string
	var offset int

	cmd := &cobra.Command{
		Use:   "list <list-name>",
		Short: "A current (or dated) best-seller list",
		Long: `Fetch a single best-seller list by its encoded name.

By default the current list is returned. Pass --date YYYY-MM-DD to fetch the
list as published nearest that date. Run "nyt books lists" to discover names.`,
		Example: `  nyt books list hardcover-fiction
  nyt books list hardcover-fiction --date 2024-01-07
  nyt books list hardcover-fiction --offset 20 --sort-by rank --sort-order ASC
  nyt books list hardcover-fiction --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			listName := strings.TrimSpace(args[0])
			if listName == "" {
				return fmt.Errorf("a list name is required; run \"nyt books lists\" to discover names")
			}
			if err := booksValidateSortOrder(sortOrder); err != nil {
				return err
			}
			sortOrder = strings.ToUpper(strings.TrimSpace(sortOrder))

			var path string
			if date != "" {
				path = fmt.Sprintf("/svc/books/v3/lists/%s/%s.json",
					url.PathEscape(date), url.PathEscape(listName))
			} else {
				path = fmt.Sprintf("/svc/books/v3/lists/current/%s.json",
					url.PathEscape(listName))
			}

			q := url.Values{}
			addInt(q, "offset", offset)
			addStr(q, "sort-by", sortBy)
			addStr(q, "sort-order", sortOrder)

			c, err := apiClient()
			if err != nil {
				return err
			}
			raw, err := c.GetRaw(ctxOf(cmd), path, q)
			if err != nil {
				return err
			}
			if jsonOutput() {
				return output.PrettyJSON(cmd.OutOrStdout(), raw)
			}
			var resp booksListResponse
			if err := unmarshal(raw, &resp); err != nil {
				return err
			}
			renderBooksList(cmd.OutOrStdout(), listName, &resp)
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "list published nearest this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&offset, "offset", 0, "results offset (must be a multiple of 20)")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "field to sort by (e.g. rank, title, author)")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "", "sort order: ASC or DESC")
	return cmd
}

func renderBooksList(w io.Writer, listName string, resp *booksListResponse) {
	if len(resp.Results.Books) == 0 {
		fmt.Fprintln(w, "No books found.")
		return
	}
	title := listName
	if resp.Results.DisplayName != "" {
		title = resp.Results.DisplayName
	}
	output.Header(w, fmt.Sprintf("Best sellers — %s (week of %s)",
		title, output.Dash(resp.Results.BestsellersDate)))
	tw := output.NewTable(w)
	fmt.Fprintln(tw, "#\tRANK\tWKS\tTITLE\tAUTHOR")
	for i, b := range resp.Results.Books {
		fmt.Fprintf(tw, "%d\t%d\t%d\t%s\t%s\n",
			i+1,
			b.Rank,
			b.WeeksOnList,
			output.Dash(output.Truncate(b.Title, 70)),
			output.Dash(output.Truncate(b.Author, 24)),
		)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d books. Use --json for full details (ISBNs, publishers).\n", len(resp.Results.Books))
}

// ---------------------------------------------------------------------------
// books overview
// ---------------------------------------------------------------------------

func newBooksOverviewCmd() *cobra.Command {
	var date string

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Top 5 of every best-seller list for a date",
		Long: `Fetch the top books from every best-seller list.

By default the most recent overview is returned. Pass --date YYYY-MM-DD to fetch
the overview published nearest that date.`,
		Example: `  nyt books overview
  nyt books overview --date 2024-01-07
  nyt books overview --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			addStr(q, "published_date", date)

			c, err := apiClient()
			if err != nil {
				return err
			}
			raw, err := c.GetRaw(ctxOf(cmd), "/svc/books/v3/lists/overview.json", q)
			if err != nil {
				return err
			}
			if jsonOutput() {
				return output.PrettyJSON(cmd.OutOrStdout(), raw)
			}
			var resp booksOverviewResponse
			if err := unmarshal(raw, &resp); err != nil {
				return err
			}
			renderBooksOverview(cmd.OutOrStdout(), &resp)
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "overview published nearest this date (YYYY-MM-DD)")
	return cmd
}

func renderBooksOverview(w io.Writer, resp *booksOverviewResponse) {
	if len(resp.Results.Lists) == 0 {
		fmt.Fprintln(w, "No lists found.")
		return
	}
	fmt.Fprintf(w, "Best-seller overview — published %s\n\n",
		output.Dash(resp.Results.PublishedDate))
	for _, list := range resp.Results.Lists {
		output.Header(w, output.Dash(list.DisplayName))
		if len(list.Books) == 0 {
			fmt.Fprintln(w, "  (no books)")
			fmt.Fprintln(w)
			continue
		}
		tw := output.NewTable(w)
		fmt.Fprintln(tw, "RANK\tTITLE\tAUTHOR")
		for _, b := range list.Books {
			fmt.Fprintf(tw, "%d\t%s\t%s\n",
				b.Rank,
				output.Dash(output.Truncate(b.Title, 70)),
				output.Dash(output.Truncate(b.Author, 24)),
			)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}
}

// ---------------------------------------------------------------------------
// shared validation
// ---------------------------------------------------------------------------

// booksValidateSortOrder rejects a --sort-order value that is not ASC or DESC.
func booksValidateSortOrder(order string) error {
	switch strings.ToUpper(strings.TrimSpace(order)) {
	case "", "ASC", "DESC":
		return nil
	default:
		return fmt.Errorf("invalid --sort-order %q: valid values are ASC, DESC", order)
	}
}

func init() {
	rootCmd.AddCommand(newBooksCmd())
}
