// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `sync` pulls the full user state from Expensify's ReconnectApp and upserts
// into the local SQLite store. The local store powers offline search, rollups,
// damage, and dupes.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var syncAll bool
	var sinceDate, policyID string
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Pull reports, expenses, and workspaces from Expensify into the local store",
		Example: "  expensify-pp-cli sync\n  expensify-pp-cli sync --since 2026-01-01",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post("/ReconnectApp", map[string]any{})
			if err != nil {
				return classifyAPIError(err)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("ReconnectApp returned HTTP %d", status))
			}

			st, err := store.Open("")
			if err != nil {
				return configErr(fmt.Errorf("opening local store: %w", err))
			}
			defer st.Close()

			if syncAll {
				fmt.Fprintln(os.Stderr, "sync: --all accepted (no-op: ReconnectApp already returns full state)")
			}
			if sinceDate != "" {
				fmt.Fprintf(os.Stderr, "sync: --since %s accepted (filter applied after upsert)\n", sinceDate)
			}
			if policyID != "" {
				fmt.Fprintf(os.Stderr, "sync: --policy %s accepted (filter applied after upsert)\n", policyID)
			}

			nReports, nExpenses, nWorkspaces, nPeople := ingestReconnectApp(st, data, sinceDate, policyID, c.Config)

			fmt.Fprintf(cmd.OutOrStdout(),
				"Synced %d reports, %d expenses, %d workspaces, %d people from Expensify.\n",
				nReports, nExpenses, nWorkspaces, nPeople)
			return nil
		},
	}
	cmd.Flags().BoolVar(&syncAll, "all", false, "Full sync (accepted for parity; ReconnectApp already returns full state)")
	cmd.Flags().StringVar(&sinceDate, "since", "", "Only upsert expenses dated on/after this YYYY-MM-DD")
	cmd.Flags().StringVar(&policyID, "policy", "", "Only upsert rows for this policy ID")
	return cmd
}

// ingestReconnectApp parses the response blob from /ReconnectApp and upserts
// every plausible report / expense / workspace / person we can find.
// Expensify's shape varies: the payload typically has `onyxData` which is an
// array of patches, each carrying `key` + `value`. Keys like `transactions_*`,
// `reports_*`, `policy_*`, `personalDetailsList`, and `session` each drive
// their own upsert path. If cfg is non-nil and its ExpensifyAccountID field
// is unset, the session.accountID discovered here is persisted to the config
// file.
func ingestReconnectApp(st *store.Store, data json.RawMessage, since, policyFilter string, cfg *config.Config) (nReports, nExpenses, nWorkspaces, nPeople int) {
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		fmt.Fprintf(os.Stderr, "sync: could not parse response JSON: %v\n", err)
		return
	}

	// ReconnectApp commonly wraps everything in {onyxData: [...]}
	if arr, ok := top["onyxData"].([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key, _ := m["key"].(string)
			val := m["value"]
			nR, nE, nW, nP := ingestOnyxSlice(st, key, val, since, policyFilter, cfg)
			nReports += nR
			nExpenses += nE
			nWorkspaces += nW
			nPeople += nP
		}
	}

	// Also try top-level shortcuts that some responses include.
	if v, ok := top["reports"]; ok {
		nR, _, _, _ := ingestOnyxSlice(st, "reports", v, since, policyFilter, cfg)
		nReports += nR
	}
	if v, ok := top["transactions"]; ok {
		_, nE, _, _ := ingestOnyxSlice(st, "transactions", v, since, policyFilter, cfg)
		nExpenses += nE
	}
	if v, ok := top["policies"]; ok {
		_, _, nW, _ := ingestOnyxSlice(st, "policies", v, since, policyFilter, cfg)
		nWorkspaces += nW
	}
	if v, ok := top["personalDetailsList"]; ok {
		_, _, _, nP := ingestOnyxSlice(st, "personalDetailsList", v, since, policyFilter, cfg)
		nPeople += nP
	}
	if v, ok := top["session"]; ok {
		maybeCaptureSessionAccountID(v, cfg)
	}
	return
}

func ingestOnyxSlice(st *store.Store, key string, val any, since, policyFilter string, cfg *config.Config) (nReports, nExpenses, nWorkspaces, nPeople int) {
	switch {
	case strings.HasPrefix(key, "transactions") || strings.HasPrefix(key, "transaction_"):
		nExpenses = upsertTransactions(st, val, since, policyFilter)
	case strings.HasPrefix(key, "reports") || strings.HasPrefix(key, "report_"):
		nReports = upsertReports(st, val, policyFilter)
	case strings.HasPrefix(key, "policies") || strings.HasPrefix(key, "policy_"):
		nWorkspaces = upsertPolicies(st, val)
	case key == "personalDetailsList":
		nPeople = upsertPersonalDetails(st, val)
	case key == "session":
		maybeCaptureSessionAccountID(val, cfg)
	}
	return
}

// upsertPersonalDetails walks a map keyed by stringified accountID, where each
// value is a `{displayName, login, avatar}` map, and upserts a Person row per
// entry. Returns the number of rows successfully upserted.
func upsertPersonalDetails(st *store.Store, val any) int {
	m, ok := val.(map[string]any)
	if !ok {
		return 0
	}
	count := 0
	for k, child := range m {
		id, err := strconv.ParseInt(strings.TrimSpace(k), 10, 64)
		if err != nil || id == 0 {
			continue
		}
		details, ok := child.(map[string]any)
		if !ok {
			continue
		}
		p := store.Person{
			AccountID:   id,
			DisplayName: firstString(details, "displayName", "display_name"),
			Login:       firstString(details, "login", "email"),
			Avatar:      firstString(details, "avatar", "avatarURL"),
		}
		if err := st.UpsertPerson(p); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert person %d: %v\n", id, err)
			continue
		}
		count++
	}
	return count
}

// maybeCaptureSessionAccountID extracts `accountID` from the session value
// (which may be a map or a nested onyx shape) and persists it to cfg when
// cfg.ExpensifyAccountID is unset.
func maybeCaptureSessionAccountID(val any, cfg *config.Config) {
	if cfg == nil || cfg.ExpensifyAccountID != 0 {
		return
	}
	m, ok := val.(map[string]any)
	if !ok {
		return
	}
	id := firstInt64(m, "accountID", "account_id", "currentUserAccountID")
	if id == 0 {
		return
	}
	cfg.ExpensifyAccountID = id
	if err := cfg.SaveAccountID(id); err != nil {
		fmt.Fprintf(os.Stderr, "sync: could not persist accountID to config: %v\n", err)
	}
}

func upsertTransactions(st *store.Store, val any, since, policyFilter string) int {
	count := 0
	process := func(raw map[string]any) {
		e := transactionFromMap(raw)
		if e.TransactionID == "" {
			return
		}
		if since != "" && e.Date != "" && e.Date < since {
			return
		}
		if policyFilter != "" && e.PolicyID != policyFilter {
			return
		}
		if err := st.UpsertExpense(e); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert expense %s: %v\n", e.TransactionID, err)
			return
		}
		count++
	}
	walkMaps(val, process)
	return count
}

func upsertReports(st *store.Store, val any, policyFilter string) int {
	count := 0
	process := func(raw map[string]any) {
		r := reportFromMap(raw)
		if r.ReportID == "" {
			return
		}
		if policyFilter != "" && r.PolicyID != policyFilter {
			return
		}
		if err := st.UpsertReport(r); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert report %s: %v\n", r.ReportID, err)
			return
		}
		count++
	}
	walkMaps(val, process)
	return count
}

func upsertPolicies(st *store.Store, val any) int {
	count := 0
	process := func(raw map[string]any) {
		w := workspaceFromMap(raw)
		if w.ID == "" {
			return
		}
		if err := st.UpsertWorkspace(w); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert workspace %s: %v\n", w.ID, err)
			return
		}
		count++
	}
	walkMaps(val, process)
	return count
}

// walkMaps handles both "val is a single object" and "val is a map of id->object"
// and "val is a slice of objects".
func walkMaps(val any, fn func(map[string]any)) {
	switch v := val.(type) {
	case map[string]any:
		// If every child is a map, treat as id->object.
		allMaps := len(v) > 0
		for _, child := range v {
			if _, ok := child.(map[string]any); !ok {
				allMaps = false
				break
			}
		}
		if allMaps {
			for _, child := range v {
				if m, ok := child.(map[string]any); ok {
					fn(m)
				}
			}
		} else {
			fn(v)
		}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				fn(m)
			}
		}
	}
}

func transactionFromMap(m map[string]any) store.Expense {
	raw, _ := json.Marshal(m)
	return store.Expense{
		TransactionID: firstString(m, "transactionID", "transaction_id", "id", "iouTransactionID"),
		ReportID:      firstString(m, "reportID", "report_id", "iouReportID"),
		Merchant:      firstString(m, "merchant", "description"),
		Amount:        firstInt64(m, "amount", "modifiedAmount"),
		Currency:      firstString(m, "currency", "modifiedCurrency"),
		Category:      firstString(m, "category"),
		Tag:           firstString(m, "tag"),
		Date:          normalizeDate(firstString(m, "created", "date", "modifiedCreated")),
		Comment:       firstString(m, "comment"),
		Receipt:       firstString(m, "receipt", "receiptPath", "filename"),
		PolicyID:      firstString(m, "policyID", "policy_id"),
		Created:       firstString(m, "created"),
		Billable:      firstBool(m, "billable"),
		Reimbursable:  firstBool(m, "reimbursable"),
		RawJSON:       string(raw),
	}
}

func reportFromMap(m map[string]any) store.Report {
	raw, _ := json.Marshal(m)
	return store.Report{
		ReportID:     firstString(m, "reportID", "report_id", "id"),
		PolicyID:     firstString(m, "policyID", "policy_id"),
		Title:        firstString(m, "reportName", "title", "name"),
		Status:       firstString(m, "stateNum", "state", "status", "statusNum"),
		Total:        firstInt64(m, "total"),
		Currency:     firstString(m, "currency"),
		Created:      firstString(m, "created", "lastActionCreated"),
		LastUpdated:  firstString(m, "lastModified", "lastUpdatedTime"),
		ExpenseCount: int(firstInt64(m, "transactionCount", "expenseCount")),
		RawJSON:      string(raw),
	}
}

func workspaceFromMap(m map[string]any) store.Workspace {
	raw, _ := json.Marshal(m)
	return store.Workspace{
		ID:         firstString(m, "id", "policyID"),
		Name:       firstString(m, "name"),
		Type:       firstString(m, "type"),
		Role:       firstString(m, "role"),
		OwnerEmail: firstString(m, "owner", "ownerEmail"),
		RawJSON:    string(raw),
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch s := v.(type) {
			case string:
				if s != "" {
					return s
				}
			case float64:
				return fmt.Sprintf("%d", int64(s))
			}
		}
	}
	return ""
}

func firstInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case float64:
				return int64(n)
			case int64:
				return n
			case int:
				return int64(n)
			case string:
				if n == "" {
					continue
				}
				var i int64
				fmt.Sscanf(n, "%d", &i)
				if i != 0 {
					return i
				}
			}
		}
	}
	return 0
}

func firstBool(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch b := v.(type) {
			case bool:
				return b
			case string:
				return b == "true" || b == "1"
			case float64:
				return b != 0
			}
		}
	}
	return false
}

// normalizeDate returns a YYYY-MM-DD slice when the input starts with that
// shape, else returns the original (empty strings pass through).
func normalizeDate(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}
