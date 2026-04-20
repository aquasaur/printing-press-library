// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// List expenses from the local store (populated by `sync`). Expensify's
// /Search dispatcher uses an undocumented filter DSL; the local store
// gives us reliable cross-year queries instead.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"
)

func newExpenseListCmd(flags *rootFlags) *cobra.Command {
	var policyID string
	var status string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your expenses from the local cache (populated by `sync`)",
		Example: `  expensify-pp-cli expense list
  expensify-pp-cli expense list --policy-id POLICY_ID_HERE --limit 20
  expensify-pp-cli expense list --json`,
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

			items, err := db.ListExpenses(filters)
			if err != nil {
				return apiErr(fmt.Errorf("listing expenses: %w", err))
			}
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			if len(items) == 0 {
				fmt.Fprintln(os.Stderr, "No expenses in local cache. Run `expensify-pp-cli sync` to fetch from Expensify.")
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(items)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			// Human table
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "TX_ID\tDATE\tMERCHANT\tAMOUNT\tCATEGORY\tREPORT")
			for _, e := range items {
				merch := e.Merchant
				if merch == "" {
					merch = "(none)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%s\t%s\n",
					e.TransactionID, e.Date, truncate(merch, 30),
					float64(e.Amount)/100, e.Category, e.ReportID)
			}
			w.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d expense(s).\n", len(items))
			return nil
		},
	}

	cmd.Flags().StringVar(&policyID, "policy-id", "", "Filter to a specific workspace")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (draft, submitted, approved, paid)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max expenses to return (0 = unlimited)")

	return cmd
}
