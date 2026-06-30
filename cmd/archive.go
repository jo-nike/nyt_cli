package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jo-nike/nyt_cli/internal/output"
	"github.com/spf13/cobra"
)

// Archive API year bounds. The NYT Archive begins in September 1851; the upper
// bound is generous so the CLI does not need updating each year.
const (
	archiveMinYear = 1851
	archiveMaxYear = 2100
)

// archiveResponse models /svc/archive/v1/{year}/{month}.json.
type archiveResponse struct {
	Status    string             `json:"status"`
	Copyright string             `json:"copyright"`
	Response  archiveResponseDoc `json:"response"`
}

type archiveResponseDoc struct {
	Meta archiveMeta  `json:"meta"`
	Docs []archiveDoc `json:"docs"`
}

type archiveMeta struct {
	Hits int `json:"hits"`
}

type archiveDoc struct {
	WebURL         string          `json:"web_url"`
	Snippet        string          `json:"snippet"`
	Abstract       string          `json:"abstract"`
	PubDate        string          `json:"pub_date"`
	DocumentType   string          `json:"document_type"`
	TypeOfMaterial string          `json:"type_of_material"`
	SectionName    string          `json:"section_name"`
	SubsectionName string          `json:"subsection_name"`
	WordCount      int             `json:"word_count"`
	Headline       archiveHeadline `json:"headline"`
	Byline         archiveByline   `json:"byline"`
}

type archiveHeadline struct {
	Main   string `json:"main"`
	Kicker string `json:"kicker"`
}

type archiveByline struct {
	Original string `json:"original"`
}

func newArchiveCmd() *cobra.Command {
	var archiveLimit int
	var archiveSection string

	cmd := &cobra.Command{
		Use:   "archive <year> <month>",
		Short: "All NYT articles for a given month (Archive API)",
		Long: `List every article the New York Times published in a given month.

The year and month are required. Year must be between 1851 and 2100; month is
1-12 (a leading zero is fine, e.g. "09" and "9" are equivalent).

Archive responses are large (a single month can be ~20MB), so big months may
exceed the default per-request timeout. Raise it with --timeout, for example:

  nyt archive 2020 1 --timeout 120s`,
		Example: `  nyt archive 2024 3
  nyt archive 2020 1 --timeout 120s
  nyt archive 2024 3 --limit 50
  nyt archive 2024 3 --section "Sports"
  nyt archive 2024 3 --limit 0 --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			year, err := archiveParseYear(args[0])
			if err != nil {
				return err
			}
			month, err := archiveParseMonth(args[1])
			if err != nil {
				return err
			}

			c, err := apiClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/svc/archive/v1/%d/%d.json", year, month)

			raw, err := c.GetRaw(ctxOf(cmd), path, nil)
			if err != nil {
				return err
			}
			if jsonOutput() {
				return output.PrettyJSON(cmd.OutOrStdout(), raw)
			}

			var resp archiveResponse
			if err := unmarshal(raw, &resp); err != nil {
				return err
			}
			renderArchive(cmd.OutOrStdout(), &resp, year, month, archiveLimit, archiveSection)
			return nil
		},
	}

	cmd.Flags().IntVar(&archiveLimit, "limit", 20, "max rows to display in the table (0 or negative = all)")
	cmd.Flags().StringVar(&archiveSection, "section", "", "filter rows by section_name (case-insensitive substring)")
	return cmd
}

// archiveParseYear validates a year argument.
func archiveParseYear(s string) (int, error) {
	year, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("invalid year %q: must be a number between %d and %d", s, archiveMinYear, archiveMaxYear)
	}
	if year < archiveMinYear || year > archiveMaxYear {
		return 0, fmt.Errorf("year %d out of range: must be between %d and %d", year, archiveMinYear, archiveMaxYear)
	}
	return year, nil
}

// archiveParseMonth validates a month argument, accepting a leading zero.
func archiveParseMonth(s string) (int, error) {
	month, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("invalid month %q: must be a number between 1 and 12", s)
	}
	if month < 1 || month > 12 {
		return 0, fmt.Errorf("month %d out of range: must be between 1 and 12", month)
	}
	return month, nil
}

func renderArchive(w io.Writer, resp *archiveResponse, year, month, limit int, section string) {
	docs := resp.Response.Docs
	if section != "" {
		needle := strings.ToLower(strings.TrimSpace(section))
		filtered := make([]archiveDoc, 0, len(docs))
		for _, d := range docs {
			if strings.Contains(strings.ToLower(d.SectionName), needle) {
				filtered = append(filtered, d)
			}
		}
		docs = filtered
	}

	output.Header(w, fmt.Sprintf("Archive %04d-%02d — %d articles", year, month, resp.Response.Meta.Hits))

	if len(docs) == 0 {
		if section != "" {
			fmt.Fprintf(w, "No articles match section filter %q.\n", section)
		} else {
			fmt.Fprintln(w, "No articles found.")
		}
		return
	}

	shown := docs
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
	}

	tw := output.NewTable(w)
	fmt.Fprintln(tw, "#\tDATE\tSECTION\tHEADLINE")
	for i, d := range shown {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
			i+1,
			output.Dash(archiveDate(d.PubDate)),
			output.Dash(output.Truncate(d.SectionName, 18)),
			output.Dash(output.Truncate(d.Headline.Main, 70)),
		)
	}
	tw.Flush()

	fmt.Fprintf(w, "\nShowing %d of %d article(s)", len(shown), len(docs))
	if section != "" {
		fmt.Fprintf(w, " matching section %q", section)
	}
	fmt.Fprintf(w, "; %d total hits this month. Use --limit 0 for all rows, --json for full details.\n", resp.Response.Meta.Hits)
}

// archiveDate returns the YYYY-MM-DD prefix of an ISO8601 pub_date. Some
// archive entries are placeholders with a zero-value date; those render as "-".
func archiveDate(pubDate string) string {
	pubDate = strings.TrimSpace(pubDate)
	if len(pubDate) >= 10 {
		pubDate = pubDate[:10]
	}
	if pubDate == "0001-01-01" {
		return ""
	}
	return pubDate
}

func init() {
	rootCmd.AddCommand(newArchiveCmd())
}
