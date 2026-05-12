// Hand-authored (not generated): `url check` — is this URL indexed by Bing,
// and if not, why? Joins GetUrlInfo with GetCrawlIssues and decodes the
// crawl-issue bitflags into a plain-English reason.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type urlCheckReport struct {
	SiteUrl         string   `json:"siteUrl"`
	Url             string   `json:"url"`
	Indexed         bool     `json:"indexed"`
	HttpStatus      int      `json:"httpStatus"`
	LastCrawledDate string   `json:"lastCrawledDate,omitempty"`
	DiscoveryDate   string   `json:"discoveryDate,omitempty"`
	DocumentSize    int      `json:"documentSize"`
	AnchorCount     int      `json:"anchorCount"`
	CrawlIssues     []string `json:"crawlIssues"`
	Reason          string   `json:"reason"`
}

func newURLCheckCmd(flags *rootFlags) *cobra.Command {
	var flagSite string

	cmd := &cobra.Command{
		Use:   "check [url]",
		Short: "Check whether a URL is indexed by Bing, with a plain-English reason if not",
		Long: `check joins GetUrlInfo with GetCrawlIssues for a single URL: it reports the
HTTP status, last-crawled and discovery dates, and — when the URL is not
indexed — decodes the matching crawl-issue bitflags into a human-readable
reason.`,
		Example:     "  bing-webmaster-tools-pp-cli url check https://example.com/page --site-url https://example.com --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var target string
			if len(args) > 0 {
				target = args[0]
			}
			site := flagSite
			if site == "" && target != "" {
				site = siteRootOf(target)
			}
			if target == "" || site == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("a URL is required (positional), plus --site-url unless the URL is a full https:// URL"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			p := map[string]string{"siteUrl": site, "url": target}
			rep := urlCheckReport{SiteUrl: site, Url: target}

			info, err := getObject(c, "/GetUrlInfo", p)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rep.HttpStatus = getInt(info, "HttpStatus")
			rep.LastCrawledDate = getStr(info, "LastCrawledDate")
			rep.DiscoveryDate = getStr(info, "DiscoveryDate")
			rep.DocumentSize = getInt(info, "DocumentSize")
			rep.AnchorCount = getInt(info, "AnchorCount")
			rep.Indexed = rep.LastCrawledDate != "" && (rep.HttpStatus == 0 || (rep.HttpStatus >= 200 && rep.HttpStatus < 300))

			if issues, err := getList(c, "/GetCrawlIssues", map[string]string{"siteUrl": site}); err == nil {
				for _, r := range issues {
					if !sameURL(getStr(r, "Url"), target) {
						continue
					}
					rep.CrawlIssues = append(rep.CrawlIssues, crawlIssueCategories(getInt(r, "Issues"))...)
				}
			}

			switch {
			case rep.Indexed && len(rep.CrawlIssues) == 0:
				rep.Reason = "URL appears indexed by Bing; no crawl issues recorded."
			case rep.Indexed:
				rep.Reason = "URL appears indexed, but has crawl issues: " + strings.Join(rep.CrawlIssues, ", ")
			case len(rep.CrawlIssues) > 0:
				rep.Reason = "URL not confirmed indexed. Crawl issues: " + strings.Join(rep.CrawlIssues, ", ")
			case rep.HttpStatus >= 400:
				rep.Reason = fmt.Sprintf("URL not indexed: Bingbot last saw HTTP %d.", rep.HttpStatus)
			case rep.LastCrawledDate == "":
				rep.Reason = "URL not indexed: Bing has not crawled it yet. Submit it (`submit url`) or check your sitemap."
			default:
				rep.Reason = "URL not confirmed indexed; no specific crawl issue recorded."
			}
			return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
		},
	}
	cmd.Flags().StringVar(&flagSite, "site-url", "", "Site URL the page belongs to (inferred from the URL when omitted)")
	return cmd
}

// siteRootOf returns the scheme://host root of a URL, which Bing Webmaster
// Tools uses as the siteUrl key.
func siteRootOf(raw string) string {
	raw = strings.TrimSpace(raw)
	i := strings.Index(raw, "://")
	if i < 0 {
		return ""
	}
	rest := raw[i+3:]
	host := rest
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		host = rest[:j]
	}
	if host == "" {
		return ""
	}
	return raw[:i+3] + host + "/"
}

func sameURL(a, b string) bool {
	return normalizeURLKey(a) == normalizeURLKey(b)
}
