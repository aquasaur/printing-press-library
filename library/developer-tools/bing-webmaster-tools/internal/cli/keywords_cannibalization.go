// Hand-authored (not generated): `keywords cannibalization` — queries where
// 2+ of a site's own pages both rank, ranked by split-impression cost.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type cannibalPage struct {
	Page        string  `json:"page"`
	Impressions int     `json:"impressions"`
	Clicks      int     `json:"clicks"`
	AvgPosition float64 `json:"avgPosition"`
}

type cannibalRow struct {
	Query            string         `json:"query"`
	PageCount        int            `json:"pageCount"`
	TotalImpressions int            `json:"totalImpressions"`
	TotalClicks      int            `json:"totalClicks"`
	Pages            []cannibalPage `json:"pages"`
}

func newKeywordsCannibalizationCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var minPages int

	cmd := &cobra.Command{
		Use:   "cannibalization [site-url]",
		Short: "Queries where two or more of your pages both rank in Bing, ranked by impression cost",
		Long: `cannibalization reads GetQueryPageStats for a site, groups rows by query,
and surfaces queries where two or more of your own pages both rank — the
classic keyword-cannibalization signal. Rows are ranked by total impressions
across the competing pages. The Bing API has no endpoint that groups pages by
query like this.`,
		Example:     "  bing-webmaster-tools-pp-cli keywords cannibalization --site-url https://example.com --json",
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
			rows, err := getList(c, "/GetQueryPageStats", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			byQuery := map[string]*cannibalRow{}
			for _, r := range rows {
				q := firstStr(r, "Query")
				page := firstStr(r, "Page", "Url")
				if q == "" || page == "" {
					continue
				}
				cr := byQuery[q]
				if cr == nil {
					cr = &cannibalRow{Query: q}
					byQuery[q] = cr
				}
				imp := getInt(r, "Impressions")
				clk := getInt(r, "Clicks")
				cr.Pages = append(cr.Pages, cannibalPage{
					Page:        page,
					Impressions: imp,
					Clicks:      clk,
					AvgPosition: round2(firstNum(r, "AvgClickPosition", "AvgImpressionPosition", "Position")),
				})
				cr.TotalImpressions += imp
				cr.TotalClicks += clk
			}
			out := make([]cannibalRow, 0, len(byQuery))
			for _, cr := range byQuery {
				cr.PageCount = len(cr.Pages)
				if cr.PageCount < minPages {
					continue
				}
				sort.SliceStable(cr.Pages, func(i, j int) bool { return cr.Pages[i].Impressions > cr.Pages[j].Impressions })
				out = append(out, *cr)
			}
			sortRowsDesc(out, func(r cannibalRow) float64 { return float64(r.TotalImpressions) })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSite, "site-url", "", "Site URL")
	cmd.Flags().IntVar(&minPages, "min-pages", 2, "Minimum distinct pages ranking for a query to flag it")
	return cmd
}
