// Hand-authored (not generated): `crawl triage` — decode crawl-issue bitflags
// into named categories, group, and weight by page traffic.
package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

type crawlTriageRow struct {
	Category            string   `json:"category"`
	Count               int      `json:"count"`
	AffectedClicks      int      `json:"affectedClicks"`
	AffectedImpressions int      `json:"affectedImpressions"`
	SampleURLs          []string `json:"sampleUrls"`
}

func newCrawlTriageCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var sampleSize int

	cmd := &cobra.Command{
		Use:   "triage [site-url]",
		Short: "Decode crawl-issue bitflags into categories, grouped and weighted by page traffic",
		Long: `triage reads GetCrawlIssues for a site, decodes each row's CrawlIssueType
bitflags into named categories, and groups by category. It also pulls
GetPageStats and joins on URL so categories affecting high-traffic pages rank
first — turning a flat error dump into a prioritized worklist.`,
		Example:     "  bing-webmaster-tools-pp-cli crawl triage --site-url https://example.com --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			site := flagSite
			if site == "" && len(args) > 0 {
				site = args[0]
			}
			if site == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("a site URL is required: pass --site-url or a positional argument"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			p := map[string]string{"siteUrl": site}
			issues, err := getList(c, "/GetCrawlIssues", p)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Build a page→traffic map (best-effort; ignored if unavailable).
			clicksByURL := map[string]int{}
			impByURL := map[string]int{}
			if pages, err := getList(c, "/GetPageStats", p); err == nil {
				for _, r := range pages {
					u := normalizeURLKey(firstStr(r, "Page", "Url", "Query"))
					if u == "" {
						continue
					}
					clicksByURL[u] += getInt(r, "Clicks")
					impByURL[u] += getInt(r, "Impressions")
				}
			}

			type agg struct {
				count   int
				clicks  int
				imp     int
				samples []string
			}
			byCat := map[string]*agg{}
			for _, r := range issues {
				u := firstStr(r, "Url")
				cats := crawlIssueCategories(getInt(r, "Issues"))
				if len(cats) == 0 {
					if code := getInt(r, "HttpStatusCode"); code >= 400 {
						cats = []string{fmt.Sprintf("http_%d", code)}
					} else {
						cats = []string{"uncategorized"}
					}
				}
				key := normalizeURLKey(u)
				for _, cat := range cats {
					a := byCat[cat]
					if a == nil {
						a = &agg{}
						byCat[cat] = a
					}
					a.count++
					a.clicks += clicksByURL[key]
					a.imp += impByURL[key]
					if u != "" && len(a.samples) < sampleSize {
						a.samples = append(a.samples, u)
					}
				}
			}

			out := make([]crawlTriageRow, 0, len(byCat))
			for cat, a := range byCat {
				out = append(out, crawlTriageRow{
					Category:            cat,
					Count:               a.count,
					AffectedClicks:      a.clicks,
					AffectedImpressions: a.imp,
					SampleURLs:          a.samples,
				})
			}
			// Rank by affected clicks, then impressions, then raw count.
			sortRowsDesc(out, func(r crawlTriageRow) float64 {
				return float64(r.AffectedClicks)*1e6 + float64(r.AffectedImpressions)*1e1 + float64(r.Count)
			})
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSite, "site-url", "", "Site URL")
	cmd.Flags().IntVar(&sampleSize, "samples", 5, "Number of sample URLs to list per category")
	return cmd
}

// normalizeURLKey lowercases the host and trims a trailing slash so crawl-issue
// URLs and page-stats URLs join even when they differ cosmetically.
func normalizeURLKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(strings.ToLower(raw), "/")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.Fragment = ""
	return u.String()
}
