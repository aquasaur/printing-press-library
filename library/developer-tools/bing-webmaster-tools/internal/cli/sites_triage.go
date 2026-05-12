// Hand-authored (not generated): `sites triage` — rank every verified site
// worst-first by crawl-issue count and traffic.
package cli

import (
	"github.com/spf13/cobra"
)

type siteTriageRow struct {
	SiteUrl         string `json:"siteUrl"`
	IsVerified      bool   `json:"isVerified"`
	CrawlIssueCount int    `json:"crawlIssueCount"`
	CrawlErrors     int    `json:"crawlErrors"`
	Clicks          int    `json:"clicks"`
	Impressions     int    `json:"impressions"`
	Error           string `json:"error,omitempty"`
}

func newSitesTriageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Rank every verified site worst-first by crawl-issue count and traffic",
		Long: `triage walks every site in your account (GetUserSites), pulls crawl issues
and rank/traffic stats for each, and ranks them worst-first by crawl-issue
count then ascending clicks. There is no cross-site endpoint — this is the
account-wide "what needs attention" view.`,
		Example:     "  bing-webmaster-tools-pp-cli sites triage --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sites, err := getList(c, "/GetUserSites", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := make([]siteTriageRow, 0, len(sites))
			for _, s := range sites {
				url := firstStr(s, "Url", "SiteUrl")
				if url == "" {
					continue
				}
				row := siteTriageRow{SiteUrl: url}
				if v, ok := s["IsVerified"].(bool); ok {
					row.IsVerified = v
				}
				p := map[string]string{"siteUrl": url}
				if issues, err := getList(c, "/GetCrawlIssues", p); err == nil {
					row.CrawlIssueCount = len(issues)
				} else {
					row.Error = err.Error()
				}
				if stats, err := getList(c, "/GetCrawlStats", p); err == nil {
					for _, r := range stats {
						row.CrawlErrors += getInt(r, "CrawlErrors")
					}
				}
				if traffic, err := getList(c, "/GetRankAndTrafficStats", p); err == nil {
					for _, r := range traffic {
						row.Clicks += getInt(r, "Clicks")
						row.Impressions += getInt(r, "Impressions")
					}
				}
				rows = append(rows, row)
			}
			// Worst first: most crawl issues, then most crawl errors, then
			// fewest clicks (a busy site with errors outranks a quiet one).
			sortRowsDesc(rows, func(r siteTriageRow) float64 {
				return float64(r.CrawlIssueCount)*1e6 + float64(r.CrawlErrors)*1e3 - float64(min(r.Clicks, 999))
			})
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	return cmd
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
