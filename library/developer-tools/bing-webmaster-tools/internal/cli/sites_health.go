// Hand-authored (not generated): `sites health` — one scored summary of a
// site's Bing presence, joining five live endpoints.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type siteHealthIssue struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type siteHealthReport struct {
	SiteUrl string `json:"siteUrl"`
	// DataComplete is false when one or more of the five backing endpoints
	// failed; callers should treat Score/Grade as unreliable in that case.
	DataComplete         bool              `json:"dataComplete"`
	Score                int               `json:"score"`
	Grade                string            `json:"grade"`
	TrafficClicks        int               `json:"trafficClicks"`
	TrafficImpressions   int               `json:"trafficImpressions"`
	TrafficCTR           float64           `json:"trafficCtr"`
	CrawledPages         int               `json:"crawledPages"`
	PagesInIndex         int               `json:"pagesInIndex"`
	CrawlErrors          int               `json:"crawlErrors"`
	BlockedByRobotsTxt   int               `json:"blockedByRobotsTxt"`
	TopCrawlIssues       []siteHealthIssue `json:"topCrawlIssues"`
	InboundLinks         int               `json:"inboundLinks"`
	UrlSubmissionDaily   int               `json:"urlSubmissionDailyQuota"`
	UrlSubmissionMonthly int               `json:"urlSubmissionMonthlyQuota"`
	Notes                []string          `json:"notes"`
}

func newSitesHealthCmd(flags *rootFlags) *cobra.Command {
	var flagSite string

	cmd := &cobra.Command{
		Use:   "health [site-url]",
		Short: "One scored summary of a site's Bing health: traffic, crawl, top issues, links, quota",
		Long: `health joins GetRankAndTrafficStats, GetCrawlStats, GetCrawlIssues,
GetLinkCounts, and GetUrlSubmissionQuota into a single scored summary for one
site. The crawl-issue bitflags are decoded into named categories. No single
Bing Webmaster API call returns this composite.`,
		Example:     "  bing-webmaster-tools-pp-cli sites health --site-url https://example.com --json",
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
			rep := siteHealthReport{SiteUrl: site}
			endpointErrors := 0

			if rows, err := getList(c, "/GetRankAndTrafficStats", p); err == nil {
				for _, r := range rows {
					rep.TrafficClicks += getInt(r, "Clicks")
					rep.TrafficImpressions += getInt(r, "Impressions")
				}
				if rep.TrafficImpressions > 0 {
					rep.TrafficCTR = float64(rep.TrafficClicks) / float64(rep.TrafficImpressions)
				}
			} else {
				endpointErrors++
				rep.Notes = append(rep.Notes, "traffic stats unavailable: "+err.Error())
			}

			if rows, err := getList(c, "/GetCrawlStats", p); err == nil {
				for _, r := range rows {
					rep.CrawledPages += getInt(r, "CrawledPages")
					rep.PagesInIndex += getInt(r, "InIndex")
					rep.CrawlErrors += getInt(r, "CrawlErrors")
					rep.BlockedByRobotsTxt += getInt(r, "BlockedByRobotsTxt")
				}
			} else {
				endpointErrors++
				rep.Notes = append(rep.Notes, "crawl stats unavailable: "+err.Error())
			}

			if rows, err := getList(c, "/GetCrawlIssues", p); err == nil {
				cat := map[string]int{}
				for _, r := range rows {
					for _, name := range crawlIssueCategories(getInt(r, "Issues")) {
						cat[name]++
					}
				}
				for name, n := range cat {
					rep.TopCrawlIssues = append(rep.TopCrawlIssues, siteHealthIssue{Category: name, Count: n})
				}
				sortRowsDesc(rep.TopCrawlIssues, func(i siteHealthIssue) float64 { return float64(i.Count) })
				if len(rep.TopCrawlIssues) > 5 {
					rep.TopCrawlIssues = rep.TopCrawlIssues[:5]
				}
			} else {
				endpointErrors++
				rep.Notes = append(rep.Notes, "crawl issues unavailable: "+err.Error())
			}

			if rows, err := getList(c, "/GetLinkCounts", p); err == nil {
				for _, r := range rows {
					rep.InboundLinks += getInt(r, "Count")
				}
			} else {
				endpointErrors++
				rep.Notes = append(rep.Notes, "link counts unavailable: "+err.Error())
			}

			if obj, err := getObject(c, "/GetUrlSubmissionQuota", p); err == nil {
				rep.UrlSubmissionDaily = getInt(obj, "DailyQuota")
				rep.UrlSubmissionMonthly = getInt(obj, "MonthlyQuota")
			} else {
				endpointErrors++
				rep.Notes = append(rep.Notes, "URL submission quota unavailable: "+err.Error())
			}

			rep.DataComplete = endpointErrors == 0
			rep.Score, rep.Grade = scoreSiteHealth(&rep)
			// When most of the backing endpoints failed, the numeric score is
			// scoring absence as if it were measured zero — suppress the grade
			// so callers don't act on a confident-looking F that reflects no
			// retrieved data.
			if endpointErrors >= 3 {
				rep.Score = 0
				rep.Grade = "unknown"
				rep.Notes = append(rep.Notes, fmt.Sprintf("%d of 5 data sources failed — score and grade are not meaningful", endpointErrors))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
		},
	}
	cmd.Flags().StringVar(&flagSite, "site-url", "", "Site URL")
	return cmd
}

// scoreSiteHealth produces a 0-100 heuristic score and a letter grade. It is
// deliberately simple: it rewards having traffic, indexed pages, and inbound
// links, and penalizes a high crawl-error ratio and an exhausted submission
// quota.
func scoreSiteHealth(r *siteHealthReport) (int, string) {
	score := 100
	totalCrawl := r.CrawledPages
	if totalCrawl > 0 {
		errRatio := float64(r.CrawlErrors) / float64(totalCrawl)
		score -= int(errRatio * 40)
	}
	if r.TrafficImpressions == 0 {
		score -= 25
		r.Notes = append(r.Notes, "no search impressions in the reporting window")
	} else if r.TrafficClicks == 0 {
		score -= 10
		r.Notes = append(r.Notes, "impressions but zero clicks — title/meta opportunity")
	}
	if r.PagesInIndex == 0 {
		score -= 15
		r.Notes = append(r.Notes, "no pages reported in the Bing index")
	}
	if r.InboundLinks == 0 {
		score -= 5
	}
	if r.UrlSubmissionDaily == 0 {
		r.Notes = append(r.Notes, "URL submission daily quota is exhausted")
	}
	if len(r.TopCrawlIssues) > 0 {
		r.Notes = append(r.Notes, fmt.Sprintf("top crawl issue: %s (%d)", r.TopCrawlIssues[0].Category, r.TopCrawlIssues[0].Count))
	}
	if score < 0 {
		score = 0
	}
	switch {
	case score >= 90:
		return score, "A"
	case score >= 80:
		return score, "B"
	case score >= 70:
		return score, "C"
	case score >= 60:
		return score, "D"
	default:
		return score, "F"
	}
}
