// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `sync` pulls the full user state from Expensify's ReconnectApp and upserts
// into the local SQLite store. The local store powers offline search, rollups,
// damage, and dupes.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/expensifysearch"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

// maxHistoryMonths caps the --history-months flag. Expensify's /Search DSL
// accepts `last-N-months` tokens but rejects values >24 in practice, so we
// clamp client-side and warn to stderr rather than letting the request fail.
const maxHistoryMonths = 24

// historyReportTypes is the allowlist for /Search historical report entries.
// Expensify's snapshot may also contain chat / task / policyExpenseChat /
// policyAnnounce entries under report_* keys — we skip those.
var historyReportTypes = map[string]bool{
	"iou":            true,
	"expenseReport":  true,
	"expense-report": true,
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var syncAll bool
	var sinceDate, policyID string
	var historyMonths int
	var noHistory bool
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Pull reports, expenses, and workspaces from Expensify into the local store",
		Example: "  expensify-pp-cli sync\n  expensify-pp-cli sync --since 2026-01-01\n  expensify-pp-cli sync --history-months 6\n  expensify-pp-cli sync --no-history",
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

			// Historical report pass. ReconnectApp returns only the current
			// active report snapshot; /Search pulls paid/archived reports.
			var nHistorical int
			if !noHistory {
				months := clampHistoryMonths(historyMonths, os.Stderr)
				doSearch := func(q expensifysearch.Query) (*expensifysearch.Response, error) {
					return c.Search(q)
				}
				n, herr := runHistoricalFetch(st, doSearch, months)
				if herr != nil {
					// ReconnectApp already committed; historical fetch is
					// additive. Log to stderr and exit 0 for partial success.
					fmt.Fprintf(os.Stderr, "sync: historical fetch failed: %v\n", herr)
				}
				nHistorical = n
			}

			if nHistorical > 0 {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Synced %d reports (%d historical), %d expenses, %d workspaces, %d people from Expensify.\n",
					nReports, nHistorical, nExpenses, nWorkspaces, nPeople)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Synced %d reports, %d expenses, %d workspaces, %d people from Expensify.\n",
					nReports, nExpenses, nWorkspaces, nPeople)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&syncAll, "all", false, "Full sync (accepted for parity; ReconnectApp already returns full state)")
	cmd.Flags().StringVar(&sinceDate, "since", "", "Only upsert expenses dated on/after this YYYY-MM-DD")
	cmd.Flags().StringVar(&policyID, "policy", "", "Only upsert rows for this policy ID")
	cmd.Flags().IntVar(&historyMonths, "history-months", 12, "Months of report history to pull via /Search (max 24)")
	cmd.Flags().BoolVar(&noHistory, "no-history", false, "Skip the /Search historical pass (ReconnectApp only)")
	return cmd
}

// clampHistoryMonths bounds the --history-months flag to [1, maxHistoryMonths].
// Writes a warning to w when the original value is out of range. Returns the
// clamped value the caller should use when building the Search query.
func clampHistoryMonths(months int, w io.Writer) int {
	if months <= 0 {
		fmt.Fprintf(w, "sync: --history-months %d invalid; using 12\n", months)
		return 12
	}
	if months > maxHistoryMonths {
		fmt.Fprintf(w, "sync: --history-months %d exceeds max %d; clamping to %d\n", months, maxHistoryMonths, maxHistoryMonths)
		return maxHistoryMonths
	}
	return months
}

// runHistoricalFetch issues a /Search call via doSearch for the last N months
// of `type:expense-report` data and upserts only real expense reports into st.
// Returns the count of historical rows upserted and any error from the Search
// call. The error is returned verbatim so callers can classify SearchError
// vs network errors; a non-nil error does NOT prevent previously-committed
// ReconnectApp rows from remaining in the store.
func runHistoricalFetch(st *store.Store, doSearch func(expensifysearch.Query) (*expensifysearch.Response, error), monthsN int) (int, error) {
	if st == nil || doSearch == nil {
		return 0, nil
	}
	q := expensifysearch.Query{
		Type:                  "expense-report",
		Filters:               expensifysearch.Eq("date", fmt.Sprintf("last-%d-months", monthsN)),
		SortBy:                "date",
		SortOrder:             "desc",
		View:                  "table",
		Hash:                  rand.Intn(1<<31-1) + 1,
		ShouldCalculateTotals: true,
	}
	resp, err := doSearch(q)
	if err != nil {
		return 0, err
	}
	return ingestHistoricalSearch(st, resp), nil
}

// ingestHistoricalSearch walks a /Search response and upserts only real
// expense reports (filtering out chat / task / policyExpenseChat entries that
// the snapshot may also carry under report_* keys).
func ingestHistoricalSearch(st *store.Store, resp *expensifysearch.Response) int {
	if st == nil || resp == nil {
		return 0
	}
	count := 0
	for _, entry := range resp.OnyxData {
		if len(entry.Value) == 0 {
			continue
		}
		var val any
		if err := json.Unmarshal(entry.Value, &val); err != nil {
			continue
		}
		inner := unwrapSnapshotData(val)
		count += walkHistoricalReports(st, entry.Key, inner)
	}
	return count
}

// walkHistoricalReports iterates the inner `data` map of a snapshot entry
// (or any direct report_* map) and upserts each child that passes the
// isExpenseReport allowlist.
func walkHistoricalReports(st *store.Store, key string, inner any) int {
	count := 0
	m, ok := inner.(map[string]any)
	if !ok {
		return 0
	}
	// snapshot_<hash> wraps { report_X: {...}, transactions_Y: {...} }.
	// reports / report_<id> entries may also arrive directly (non-snapshot).
	if strings.HasPrefix(key, "report_") || key == "reports" {
		// The entry's value IS the report row itself (rare) — walk one.
		if isExpenseReport(m) {
			count += upsertHistoricalReport(st, m)
		}
		// Also handle the "id -> report" map shape.
		for _, child := range m {
			if row, ok := child.(map[string]any); ok && isExpenseReport(row) {
				count += upsertHistoricalReport(st, row)
			}
		}
		return count
	}
	// snapshot_* and everything else: iterate children looking for report_* keys.
	for k, child := range m {
		if !strings.HasPrefix(k, "report_") {
			continue
		}
		row, ok := child.(map[string]any)
		if !ok {
			continue
		}
		if !isExpenseReport(row) {
			continue
		}
		count += upsertHistoricalReport(st, row)
	}
	return count
}

// upsertHistoricalReport runs the shared reportFromMap translation and
// upserts. Returns 1 on success, 0 otherwise.
func upsertHistoricalReport(st *store.Store, row map[string]any) int {
	r := reportFromMap(row)
	if r.ReportID == "" {
		return 0
	}
	if err := st.UpsertReport(r); err != nil {
		fmt.Fprintf(os.Stderr, "sync: upsert historical report %s: %v\n", r.ReportID, err)
		return 0
	}
	return 1
}

// isExpenseReport reports whether a raw report map is a real expense report
// (and not a chat / task / onboarding / policyExpenseChat snapshot entry).
// Allowlist: type ∈ {iou, expenseReport, expense-report}. Defensive fallback:
// when type is missing AND the row has a non-empty reportName AND stateNum is
// present, treat it as a real report — some snapshot entries omit type.
func isExpenseReport(m map[string]any) bool {
	typ := strings.TrimSpace(firstString(m, "type"))
	if typ != "" {
		return historyReportTypes[typ]
	}
	// Fallback: non-empty reportName + stateNum present.
	if firstString(m, "reportName", "title", "name") == "" {
		return false
	}
	if _, has := m["stateNum"]; !has {
		return false
	}
	return true
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
		StateNum:     firstInt64(m, "stateNum", "statusNum"),
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
