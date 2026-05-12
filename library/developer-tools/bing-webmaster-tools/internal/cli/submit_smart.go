// Hand-authored (not generated): `submit smart` — quota-aware batch URL
// submission with a local ledger and a CI-friendly exit code.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster-tools/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster-tools/internal/store"
)

type submitSmartResult struct {
	SiteUrl        string   `json:"siteUrl"`
	RequestedCount int      `json:"requestedCount"`
	DailyQuota     int      `json:"dailyQuota"`
	MonthlyQuota   int      `json:"monthlyQuota"`
	SubmittedCount int      `json:"submittedCount"`
	Submitted      []string `json:"submitted"`
	DroppedCount   int      `json:"droppedCount"`
	Dropped        []string `json:"dropped"`
	DryRun         bool     `json:"dryRun"`
}

func newSubmitSmartCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var flagFile string
	var flagURLs []string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "smart [site-url]",
		Short: "Submit URLs for indexing up to the remaining daily quota, with a local ledger",
		Long: `smart reads the remaining URL-submission quota (GetUrlSubmissionQuota),
submits only up to that allowance via SubmitUrlBatch, records every submission
to the local store with a timestamp, and exits non-zero if any URL had to be
dropped because the quota ran out — so a CI pipeline can flag it.

URLs come from --url (repeatable), --file (one per line, # comments allowed),
or, when neither is given, stdin.`,
		Example:     "  bing-webmaster-tools-pp-cli submit smart --site-url https://example.com --file changed-urls.txt --json",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
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

			previewOnly := dryRunOK(flags) || cliutil.IsVerifyEnv()

			urls, err := collectSubmitURLs(cmd, flagURLs, flagFile)
			if err != nil {
				if previewOnly {
					// A missing --file under --dry-run / verify is not a real
					// error — report an empty plan rather than failing.
					return printJSONFiltered(cmd.OutOrStdout(), submitSmartResult{SiteUrl: site, DryRun: true}, flags)
				}
				return usageErr(err)
			}
			if len(urls) == 0 {
				if previewOnly {
					return printJSONFiltered(cmd.OutOrStdout(), submitSmartResult{SiteUrl: site, DryRun: true}, flags)
				}
				return usageErr(fmt.Errorf("no URLs to submit: provide --url, --file, or pipe URLs on stdin"))
			}

			res := submitSmartResult{SiteUrl: site, RequestedCount: len(urls)}

			// Verify-env / dry-run: report the plan, never call the API.
			if previewOnly {
				res.DryRun = true
				res.Submitted = urls
				res.SubmittedCount = len(urls)
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			quota, err := getObject(c, "/GetUrlSubmissionQuota", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			res.DailyQuota = getInt(quota, "DailyQuota")
			res.MonthlyQuota = getInt(quota, "MonthlyQuota")

			allow := res.DailyQuota
			if res.MonthlyQuota > 0 && res.MonthlyQuota < allow {
				allow = res.MonthlyQuota
			}
			if allow < 0 {
				allow = 0
			}
			toSubmit := urls
			if len(toSubmit) > allow {
				toSubmit = urls[:allow]
				res.Dropped = append(res.Dropped, urls[allow:]...)
			}
			res.DroppedCount = len(res.Dropped)

			if len(toSubmit) > 0 {
				body := map[string]any{"siteUrl": site, "urlList": toSubmit}
				if _, _, err := c.Post("/SubmitUrlBatch", body); err != nil {
					return classifyAPIError(err, flags)
				}
				res.Submitted = toSubmit
				res.SubmittedCount = len(toSubmit)
				recordSubmittedURLs(cmd, dbPath, site, toSubmit)
			}

			if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
				return err
			}
			if res.DroppedCount > 0 {
				return usageErr(fmt.Errorf("%d of %d URLs were not submitted: daily quota (%d) exhausted", res.DroppedCount, res.RequestedCount, res.DailyQuota))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site-url", "", "Site URL the pages belong to")
	cmd.Flags().StringVar(&flagFile, "file", "", "File of URLs to submit, one per line")
	cmd.Flags().StringArrayVar(&flagURLs, "url", nil, "URL to submit (repeatable)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for the submission ledger (default: ~/.local/share/bing-webmaster-tools-pp-cli/data.db)")
	return cmd
}

func collectSubmitURLs(cmd *cobra.Command, flagURLs []string, flagFile string) ([]string, error) {
	var raw []string
	raw = append(raw, flagURLs...)
	if flagFile != "" {
		b, err := os.ReadFile(flagFile)
		if err != nil {
			return nil, fmt.Errorf("reading --file %q: %w", flagFile, err)
		}
		raw = append(raw, strings.Split(string(b), "\n")...)
	}
	if len(flagURLs) == 0 && flagFile == "" {
		// Read from stdin only when it's not an interactive terminal.
		if in, ok := cmd.InOrStdin().(*os.File); !ok || !isTerminal(in) {
			b, err := io.ReadAll(cmd.InOrStdin())
			if err == nil {
				raw = append(raw, strings.Split(string(b), "\n")...)
			}
		}
	}
	return readURLLines(strings.Join(raw, "\n")), nil
}

// recordSubmittedURLs appends the submitted URLs to the local `submit` table
// with a timestamp so `submit url` / future tooling can warn on re-submits. A
// store failure is non-fatal — the submission already happened.
func recordSubmittedURLs(cmd *cobra.Command, dbPath, site string, urls []string) {
	if dbPath == "" {
		dbPath = defaultDBPath("bing-webmaster-tools-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open store to record submissions: %v\n", err)
		return
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, u := range urls {
		rec := map[string]any{"Url": u, "SiteUrl": site, "SubmittedAt": now, "Source": "submit smart"}
		b, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		if err := db.UpsertSubmit(b); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not record submission for %s: %v\n", u, err)
		}
	}
}
