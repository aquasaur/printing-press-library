// Hand-authored (not generated): `traffic ctr-gaps` — high-impression /
// low-CTR queries ranked by estimated lost clicks.
package cli

import (
	"fmt"
	"math"

	"github.com/spf13/cobra"
)

type ctrGapRow struct {
	Query         string  `json:"query"`
	Impressions   int     `json:"impressions"`
	Clicks        int     `json:"clicks"`
	CTR           float64 `json:"ctr"`
	AvgPosition   float64 `json:"avgPosition"`
	ExpectedCTR   float64 `json:"expectedCtr"`
	EstLostClicks float64 `json:"estLostClicks"`
}

func newTrafficCtrGapsCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var minImpressions int

	cmd := &cobra.Command{
		Use:   "ctr-gaps [site-url]",
		Short: "Queries with high impressions but low click-through, ranked by estimated lost clicks",
		Long: `ctr-gaps reads GetQueryStats for a site, keeps queries with at least
--min-impressions impressions, estimates an expected CTR from the average
position, and ranks the shortfall by estimated lost clicks. The Bing API
returns raw stats; the opportunity ranking is computed here.`,
		Example:     "  bing-webmaster-tools-pp-cli traffic ctr-gaps --site-url https://example.com --min-impressions 100 --json",
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
			rows, err := getList(c, "/GetQueryStats", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := make([]ctrGapRow, 0, len(rows))
			for _, r := range rows {
				imp := getInt(r, "Impressions")
				if imp < minImpressions {
					continue
				}
				clicks := getInt(r, "Clicks")
				pos := firstNum(r, "AvgClickPosition", "AvgImpressionPosition", "Position")
				ctr := 0.0
				if imp > 0 {
					ctr = float64(clicks) / float64(imp)
				}
				exp := expectedCTRForPosition(pos)
				if ctr >= exp {
					continue
				}
				lost := math.Round((exp-ctr)*float64(imp)*100) / 100
				out = append(out, ctrGapRow{
					Query:         firstStr(r, "Query"),
					Impressions:   imp,
					Clicks:        clicks,
					CTR:           round4(ctr),
					AvgPosition:   round2(pos),
					ExpectedCTR:   round4(exp),
					EstLostClicks: lost,
				})
			}
			sortRowsDesc(out, func(r ctrGapRow) float64 { return r.EstLostClicks })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSite, "site-url", "", "Site URL")
	cmd.Flags().IntVar(&minImpressions, "min-impressions", 50, "Minimum impressions for a query to be considered")
	return cmd
}

// expectedCTRForPosition is a rough first-page CTR curve (organic search,
// blended across studies). Positions beyond 10 fall back to a small floor.
func expectedCTRForPosition(pos float64) float64 {
	switch {
	case pos <= 0:
		return 0.05
	case pos <= 1:
		return 0.28
	case pos <= 2:
		return 0.16
	case pos <= 3:
		return 0.11
	case pos <= 4:
		return 0.08
	case pos <= 5:
		return 0.06
	case pos <= 6:
		return 0.045
	case pos <= 7:
		return 0.035
	case pos <= 8:
		return 0.03
	case pos <= 9:
		return 0.025
	case pos <= 10:
		return 0.02
	default:
		return 0.01
	}
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
func round4(f float64) float64 { return float64(int(f*10000+0.5)) / 10000 }
