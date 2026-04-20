// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// List reports from the local store (populated by `sync`).

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"
)

func newReportListCmd(flags *rootFlags) *cobra.Command {
	var policyID string
	var status string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your reports from the local cache (populated by `sync`)",
		Example: `  expensify-pp-cli report list
  expensify-pp-cli report list --status open
  expensify-pp-cli report list --policy-id POLICY_ID_HERE --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.Open(store.DefaultPath())
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w", err))
			}
			defer db.Close()
			if err := db.Migrate(); err != nil {
				return apiErr(fmt.Errorf("migrating local store: %w", err))
			}

			filters := map[string]string{}
			if policyID != "" {
				filters["policy_id"] = policyID
			}
			if status != "" {
				filters["status"] = status
			}
			items, err := db.ListReports(filters)
			if err != nil {
				return apiErr(fmt.Errorf("listing reports: %w", err))
			}
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			if len(items) == 0 {
				fmt.Fprintln(os.Stderr, "No reports in local cache. Run `expensify-pp-cli sync` to fetch from Expensify.")
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(items)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "REPORT_ID\tSTATUS\tTITLE\tTOTAL\tEXPENSES\tLAST_UPDATED")
			for _, r := range items {
				title := r.Title
				if title == "" {
					title = "(untitled)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%d\t%s\n",
					r.ReportID, r.Status, truncate(title, 30),
					float64(r.Total)/100, r.ExpenseCount, r.LastUpdated)
			}
			w.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d report(s).\n", len(items))
			return nil
		},
	}

	cmd.Flags().StringVar(&policyID, "policy-id", "", "Filter to a specific workspace")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (open, submitted, approved, reimbursed)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max reports to return (0 = unlimited)")

	return cmd
}
