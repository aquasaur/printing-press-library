// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// links_rewrite walks the local /store cache (store.Open-backed) to find links
// matching the given pattern, computes the rewritten destination, and either
// previews the diff (--dry-run) or PATCHes the API. Multi-source: store + API.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/spf13/cobra"
)

type rewriteHit struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	Key       string `json:"key"`
	ShortLink string `json:"shortLink,omitempty"`
	Before    string `json:"before"`
	After     string `json:"after"`
}

func newLinksRewriteCmd(flags *rootFlags) *cobra.Command {
	var match string
	var replace string
	var domainFilter string
	var dryRun bool
	var doApply bool

	cmd := &cobra.Command{
		Use:   "rewrite",
		Short: "Bulk URL/UTM rewrite with diff preview before sending",
		Long: `Rewrite the destination URL of every link whose URL matches a regex.
Show every link that would change AND the exact patch BEFORE sending the bulk
update. The --dry-run flag (default) makes this a safe preview; pass --apply to
actually patch the matched links via the API.

Use cases:
  - mass UTM source migration (utm_source=oldcamp -> utm_source=newcamp)
  - domain migration (https://staging.x.com -> https://x.com)
  - campaign cleanup (deleted-track parameter scrub)`,
		Example: `  # Preview a UTM source rewrite
  dub-pp-cli links rewrite --match 'utm_source=oldcamp' --replace 'utm_source=newcamp' --dry-run

  # Actually apply, scoped to one domain
  dub-pp-cli links rewrite --match 'old\.example\.com' --replace 'example.com' --domain dub.sh --apply --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if match == "" || replace == "" {
				return usageErr(fmt.Errorf("--match and --replace are required"))
			}
			re, err := regexp.Compile(match)
			if err != nil {
				return usageErr(fmt.Errorf("invalid regex --match: %w", err))
			}
			if !dryRun && !doApply && !flags.dryRun {
				return usageErr(fmt.Errorf("pass --apply (and --yes) to send the patch, or --dry-run to preview"))
			}
			previewing := dryRun || flags.dryRun || !doApply

			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			query := `SELECT id, data FROM links WHERE archived = 0 OR archived IS NULL`
			rows, err := db.DB().Query(query)
			if err != nil {
				return apiErr(fmt.Errorf("query links: %w", err))
			}
			defer rows.Close()

			hits := make([]rewriteHit, 0)
			for rows.Next() {
				var id, raw string
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				domain := stringField(obj, "domain")
				if domainFilter != "" && domain != domainFilter {
					continue
				}
				url := stringField(obj, "url")
				if !re.MatchString(url) {
					continue
				}
				after := re.ReplaceAllString(url, replace)
				if after == url {
					continue
				}
				hits = append(hits, rewriteHit{
					ID:        id,
					Domain:    domain,
					Key:       stringField(obj, "key"),
					ShortLink: stringField(obj, "shortLink"),
					Before:    url,
					After:     after,
				})
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].ID < hits[j].ID })

			if previewing {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{
						"matched": len(hits),
						"applied": false,
						"changes": hits,
					})
				}
				if len(hits) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no links matched")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d link(s) would change. Pass --apply --yes to commit.\n\n", len(hits))
				headers := []string{"ID", "DOMAIN", "KEY", "BEFORE", "AFTER"}
				rowsTbl := make([][]string, 0, len(hits))
				for _, h := range hits {
					rowsTbl = append(rowsTbl, []string{h.ID, h.Domain, h.Key, trunc(h.Before, 50), trunc(h.After, 50)})
				}
				return flags.printTable(cmd, headers, rowsTbl)
			}

			// Apply path
			if !flags.yes && !flags.noInput {
				return usageErr(fmt.Errorf("--apply requires --yes (refusing to mutate without confirmation)"))
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no links matched")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			applied := 0
			failed := 0
			for _, h := range hits {
				_, status, err := c.Patch("/links/"+h.ID, map[string]any{"url": h.After})
				if err != nil {
					failed++
					fmt.Fprintf(cmd.ErrOrStderr(), "FAIL %s (status %d): %v\n", h.ID, status, err)
					continue
				}
				applied++
			}
			summary := map[string]any{"matched": len(hits), "applied": applied, "failed": failed, "changes": hits}
			if flags.asJSON {
				return flags.printJSON(cmd, summary)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied: %d / %d (failed: %d)\n", applied, len(hits), failed)
			if failed > 0 {
				return apiErr(fmt.Errorf("%d link(s) failed to update", failed))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&match, "match", "", "Regex matched against destination URL (required)")
	cmd.Flags().StringVar(&replace, "replace", "", "Replacement string applied via regexp.ReplaceAllString (required)")
	cmd.Flags().StringVar(&domainFilter, "domain", "", "Only consider links on this short domain")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Preview only — do not patch the API (default)")
	cmd.Flags().BoolVar(&doApply, "apply", false, "Send the bulk patch (must combine with --yes)")
	return cmd
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// previewRewrite is a small pure helper used by tests.
func previewRewrite(url, match, replace string) (string, bool, error) {
	re, err := regexp.Compile(match)
	if err != nil {
		return "", false, err
	}
	if !re.MatchString(url) {
		return url, false, nil
	}
	after := re.ReplaceAllString(url, replace)
	return after, after != url, nil
}
