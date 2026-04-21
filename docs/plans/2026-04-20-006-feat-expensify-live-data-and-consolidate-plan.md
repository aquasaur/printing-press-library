---
title: "feat: Expensify CLI live-data fixes and report consolidate"
type: feat
status: active
date: 2026-04-20
---

# feat: Expensify CLI live-data fixes and report consolidate

**Target repo:** printing-press-library (branch `feat/expensify`, PR #104)

## Overview

The Expensify CLI shipped via PR #104 crumbled on real data during a live dogfood session. Six gaps were identified, each with live-captured evidence:

1. `/Search` uses an undocumented jsonQuery DSL; current `expense list` / `report list` fell back to local cache because the shape was unknown
2. Sync walker reads `ReconnectApp`'s 1-report summary instead of querying `/Search` across 12+ months of history, so paid/archived reports never land locally
3. No `ownerAccountID` filter — an admin user sees reports from every person they can view and the CLI presents them all as "theirs"
4. No accountID → name mapping — output shows bare IDs like `20647491` instead of `Myk Melez`
5. `expense quick`'s merchant parser treats "Tacos $5" as merchant=Tacos (the word before the dollar amount) rather than recognizing that "Tacos" is a category-like noun with no vendor
6. `damage` shows $0 across all status buckets because sync doesn't map Onyx's `stateNum` / `statusNum` to the CLI's status values
7. One new command: `report consolidate` to merge several draft reports into one and optionally submit it

The user has 6 small "April expenses" drafts right now that would benefit from `consolidate`; that's the flagship new feature.

## Problem Frame

Live dogfood against the user's real Expensify account surfaced that the CLI's read path doesn't work for authenticated users who are also workspace admins. The CLI presented reports owned by 5 different employees as "your reports" when answering "did my last expense report get paid?". The user correctly flagged this as a real data-safety concern. Separately, historical/paid reports weren't in the local cache at all, so any "did X get paid" question required bypassing the CLI entirely.

The evidence required to fix all 7 gaps was captured live in the session — no additional reverse engineering is needed. Specifically:

- A real `jsonQuery` from the web UI showing `type:expense-report`, filter tree `{operator, left, right}`, `action:submit from:<accountID>`, `sortBy`/`sortOrder`/`view` fields, and the `hash`/`recentSearchHash`/`similarSearchHash` bookkeeping fields
- The exact shape of `personalDetailsList` — a map keyed by accountID (string) with `{displayName, login}` values, returned by `/ReconnectApp`
- The policy `employeeList` shape — map keyed by email with `{role}` values
- The `stateNum` / `statusNum` numeric enum used across Onyx (0=OPEN, 1=SUBMITTED, 2=CLOSED/APPROVED, 3=APPROVED, 4=REIMBURSED, 5=BILLING, 6=PAID)
- A live example of the ownership problem — 26 reports pulled, grouped by `ownerAccountID`, with the user owning 10 and 5 other employees owning the rest

## Requirements Trace

- R1. `report list` and `expense list` return live data from Expensify (not only local cache) when the caller wants fresh results, using the real `/Search` jsonQuery DSL
- R2. Both commands default to `ownerAccountID == me` so admins don't see other people's reports unless they ask
- R3. Every command that surfaces an accountID also surfaces the person's name or email (e.g. "Myk Melez" instead of `20647491`)
- R4. `sync` pulls the full 12-month report history (not just the ReconnectApp summary) and filters out chat / task / onboarding reports
- R5. `expense quick "Tacos $5"` does not set `merchant="Tacos"` when "Tacos" reads like a category noun — it either prompts, defers to `--merchant`, or defaults to a safe placeholder
- R6. `damage` correctly maps `stateNum`/`statusNum` so the Expensed / Pending / Approved / Paid buckets reflect reality
- R7. `report consolidate` merges multiple draft reports into one target, optionally submitting, with a `--dry-run` preview

## Scope Boundaries

- Do NOT modify write-path request bodies for `RequestMoney`, `CreateReport`, `SubmitReport`, `AddExpensesToReport`, `PayMoneyRequest` — they're untested against live 2xx responses and changing them risks breaking what dry-run already exercises cleanly. `report consolidate` uses only `/AddExpensesToReport` (already wired) plus optional `/SubmitReport` (already wired) — it does not invent new write paths
- Do NOT un-stub `undo`, `mcp`, `close`, `admin policy-diff`, or `expense watch` — each is a meaningful feature that deserves its own plan
- Do NOT extend auth to support the Integration Server partner-key path for the admin Search — session authToken is the only auth used by this plan
- Do NOT rebuild the sync walker into a full-state engine that tracks deletions; this plan only adds upsert coverage for historical records, not lifecycle tracking

### Deferred to Separate Tasks

- Multi-workspace default picking when a user has 2+ policies: noted in Gap #10 of the session but deferred
- `report list --owner <email>` admin override flag: Unit 3 enables it structurally but the CLI flag ships in a follow-up
- Write-path shape validation against live Expensify responses: needs its own dogfood pass with consent to create test expenses

## Context & Research

### Relevant Code and Patterns

- `library/productivity/expensify/internal/client/client.go` — form-encoded request builder already routes to `www.expensify.com/api` or `integrations.expensify.com` based on path prefix (`buildNewExpensifyRequest` vs `buildIntegrationServerRequest`). Adding `/Search` support reuses the existing NewExpensify path
- `library/productivity/expensify/internal/store/store.go` — SQLite store with typed upsert helpers (`UpsertExpense`, `UpsertReport`, `UpsertWorkspace`). Unit 4 adds a sibling `UpsertPerson` plus a `people` table following the same pattern; Unit 2 adds `ListReports` variants that filter by owner
- `library/productivity/expensify/internal/cli/sync.go` — existing `ingestReconnectApp` + `walkMaps` pattern for mining Onyx payloads. Unit 5 extends this to call `/Search` for historical reports and `/ReconnectApp` for `personalDetailsList` (currently dropped on the floor)
- `library/productivity/expensify/internal/cli/expense_list.go` + `report_list.go` — both currently read from local store with no live fetch option. Unit 2 adds a `--live` flag that issues `/Search` when the caller wants fresh data; default remains local for speed
- `library/productivity/expensify/internal/cli/damage.go` — currently queries `store.Damage(month, policyID)`. Unit 6 changes the store's `Damage` method to bucket by mapped status fields, not `status=="OPEN"` string matching
- `library/productivity/expensify/internal/cli/expense_quick.go` — `quickParsed` struct and regex-driven parser. Unit 3 adds a category-noun blocklist and confidence gate
- `library/productivity/expensify/internal/cli/report.go` — parent cobra command registering report subcommands. Unit 7 adds `newReportConsolidateCmd`

### Institutional Learnings

- The `printing-press-library` repo's prior expensify-specific plan is this one; no prior institutional learning applies. Similar sync-walker patterns exist in `library/finance/yahoo-finance` (FTS5 + Onyx-style tree walking) and `library/productivity/linear` (owner-scoped queries) — both follow the "live vs local" flag convention this plan adopts

### External References

- No external research needed. The `/Search` DSL was reverse-engineered from the user's web UI traffic during the live dogfood session. The captured jsonQuery shape is in the conversation transcript; reproducing it requires capturing one real fetch via browser interception, which the implementer can do by pointing agent-browser at new.expensify.com if they need to cross-check

## Key Technical Decisions

- **Filter DSL is a typed Go struct tree, not a string builder**: The jsonQuery `filters` field is a nested `{operator, left, right}` tree. Encoding it as a struct (`type Filter struct { Operator string; Left any; Right any }`) with JSON tags lets the implementer compose filters from typed helpers (`eq("action", "submit")`, `and(f1, f2)`) rather than string concatenation. Rationale: the only mechanical bugs that aren't caught by `go vet` are typos in DSL strings; a typed tree catches them at compile time
- **`--live` flag opt-in, not default**: `expense list` / `report list` default to local store for speed. Adding `--live` issues `/Search` directly and can also upsert into the cache as a side-effect. Rationale: local is ~20ms, live is ~500ms; most agent workflows (damage / search / rollup) benefit from the cache; freshness-sensitive queries explicitly opt in
- **Owner filter default, override with `--all-visible`**: When the caller has session auth, compute their own accountID from the last ReconnectApp response (persisted in config as `expensify_account_id`) and filter `ownerAccountID == me` by default. `--all-visible` disables the filter; `--owner <email>` narrows to a specific person. Rationale: the session user owning reports is the 99% case; admins who want the wider view say so explicitly
- **`people` table is strictly a cache**: Synced from `/ReconnectApp`'s `personalDetailsList` on every sync. No mutation commands, no write path. Resolving an accountID to a display name is a local lookup with graceful fallback ("accountID:20647491" when unknown). Rationale: keeps scope tight; directory sync is Expensify's job
- **Damage status mapping uses `stateNum`, not `statusNum`**: Per the observed values, `stateNum` changes monotonically through OPEN → SUBMITTED → CLOSED/APPROVED → REIMBURSED. `statusNum` is noisier (often mirrors stateNum but sometimes lags during transitions). Rationale: `stateNum` is the canonical lifecycle field in Expensify's UI; `statusNum` is a derived/display variant
- **`report consolidate` doesn't delete source reports**: It attaches expenses from N drafts into a target report, then leaves the empty source drafts alone (they're harmless; Expensify garbage-collects empty reports or the user deletes them). Rationale: no `/DeleteReport` call means no destructive action we haven't tested; empty drafts cost nothing
- **Category noun blocklist for `expense quick`**: Maintain a small static list of category-shaped words that should NOT be treated as merchants: `tacos, lunch, dinner, breakfast, coffee, snack, drink, drinks, meal, ride, gas, fuel, tip, parking, toll`. When the prompt has no "at X" phrase and the leading word matches the blocklist, require `--merchant` or default to `"<category>"` placeholder. Rationale: clean heuristic, easy to extend, avoids NLP dependency

## Open Questions

### Resolved During Planning

- **Where does the user's own accountID come from?** → From `/ReconnectApp`'s `session.accountID` field (captured but currently discarded). Unit 4's `sync` extension persists it to `~/.config/expensify-pp-cli/config.toml` under `expensify_account_id`
- **How does `--live` interact with `--data-source live|local|auto`?** → The existing `--data-source` flag is a generator-emitted global. `--live` on a specific list command is the per-command override. When both are set, `--live` wins. When only `--data-source live` is set, list commands behave as if `--live` were passed
- **Should `report consolidate` touch open reports only, or any state?** → Drafts only (stateNum=0). Consolidating submitted/approved/paid reports is a footgun and isn't what the user's open-April-drafts situation calls for

### Deferred to Implementation

- **Exact `hash` / `recentSearchHash` / `similarSearchHash` values for `/Search` requests**: the web UI computes these client-side from the query shape. The implementer can use arbitrary positive integers — the server uses them for client-side cache keying only. Confirm via one live call; if the server rejects, compute a deterministic `hash(jsonQuery)` as a follow-up
- **Whether `sync --full` should clear the store first or only upsert**: start with upsert-only (safer, no accidental loss). Revisit if stale rows accumulate
- **Exact regex for the category-noun blocklist edge cases** (e.g., "Dinner at Maya" should parse "Maya" as merchant, not flag "Dinner"): "at X" phrase detection takes priority over the blocklist check; the blocklist only fires when no merchant phrase is present

## Implementation Units

- [ ] **Unit 1: Typed Search filter DSL + `/Search` client helper**

  **Goal:** Let any CLI command issue an Expensify `/Search` call with a typed filter tree, no string-built jsonQuery, and get back parsed onyxData.

  **Requirements:** R1

  **Dependencies:** none — foundation for Units 2 and 5

  **Files:**
  - Create: `library/productivity/expensify/internal/expensifysearch/search.go`
  - Create: `library/productivity/expensify/internal/expensifysearch/search_test.go`
  - Modify: `library/productivity/expensify/internal/client/client.go` (export a `Search()` helper that accepts a typed `Query` struct and returns the parsed response envelope)

  **Approach:**
  - Define `Query`, `Filter`, and `Response` types in the new `expensifysearch` package
  - `Query` fields mirror the captured jsonQuery: `Type`, `Status`, `SortBy`, `SortOrder`, `View`, `Filters *Filter`, `InputQuery`, `Hash`, `Offset`, `ShouldCalculateTotals`
  - `Filter` is recursive: `Operator string` (eq, and, or), `Left any`, `Right any`. Left can be a string (field name) or a nested `*Filter`; same for Right
  - Provide small constructors: `Eq(field, value)`, `And(a, b)`, `Or(a, b)` — they're trivial and make call sites readable
  - `client.Search(q Query)` marshals Q to jsonQuery, calls existing `Post("/Search", ...)` with the form-body flow, parses the envelope, returns a structured `Response` with `JSONCode int`, `Message string`, `OnyxData []OnyxEntry`
  - On jsonCode != 200, surface the message as a typed error that Unit 2's command handlers can display to the user without dumping raw JSON

  **Patterns to follow:**
  - `library/productivity/expensify/internal/client/client.go` `buildNewExpensifyRequest` for the form body shape
  - `library/productivity/expensify/internal/cli/sync.go` `walkMaps` for onyxData traversal

  **Test scenarios:**
  - Happy path: `Query{Type:"expense-report", Filters:Eq("action","submit")}` marshals to a jsonQuery string with matching fields and nested operator tree
  - Happy path: `And(Eq("action","submit"), Eq("from","20631946"))` marshals to `{"operator":"and","left":{"operator":"eq","left":"action","right":"submit"},"right":{"operator":"eq","left":"from","right":"20631946"}}`
  - Edge case: nil `Filters` serializes as `"filters":null`, not `{}` (matches web UI behavior)
  - Error path: jsonCode 401 response surfaces as a typed error with the message "Invalid Query" and the request's inputQuery for context
  - Error path: jsonCode 407 (session expired) surfaces as a typed error the CLI can classify into exit code 4 (auth)
  - Integration: a canned Expensify response with one `snapshot_` entry containing three `report_*` rows parses into three `OnyxEntry` objects, each with owner/status fields accessible

  **Verification:**
  - `go test ./internal/expensifysearch/...` passes
  - `Query{}` with no filters round-trips through marshal-then-parse with the same shape
  - Client helper dispatches via the existing `Post` path; no bypass of rate limiting or dry-run preview

- [ ] **Unit 2: `expense list` / `report list` gain `--live` mode via Search**

  **Goal:** When `--live` (or `--data-source live`) is set, both list commands issue a Search call with the right filter tree and optionally upsert results into the local cache. Owner-filter-by-default.

  **Requirements:** R1, R2

  **Dependencies:** Unit 1

  **Files:**
  - Modify: `library/productivity/expensify/internal/cli/expense_list.go`
  - Modify: `library/productivity/expensify/internal/cli/report_list.go`
  - Modify: `library/productivity/expensify/internal/config/config.go` (add `ExpensifyAccountID string` field; not set here, consumed here)
  - Test: `library/productivity/expensify/internal/cli/expense_list_test.go`
  - Test: `library/productivity/expensify/internal/cli/report_list_test.go`

  **Approach:**
  - Add `--live` bool and `--owner` string flags to both commands
  - When `--live`: build a `Query` using `type:expense` (expenses) or `type:expense-report` (reports), attach an owner filter by default (`Eq("from", cfg.ExpensifyAccountID)`), add any user-supplied filters (policyID, status, date range) via `And`
  - Merge caller flags into the filter tree; if `--all-visible` is set, omit the owner filter
  - After a successful Search, extract report/expense rows from `onyxData[*].snapshot_<hash>.data.report_*` (reports) or `...transactions_*` (expenses), upsert to local store, and return for display
  - When `--live` is NOT set, fall back to the current local-store path

  **Patterns to follow:**
  - Existing local-store read path in both files
  - `library/productivity/expensify/internal/cli/sync.go` for the onyxData snapshot walk

  **Test scenarios:**
  - Happy path: `expense list --live` with a configured `ExpensifyAccountID` builds a Query whose filter tree is `And(Eq("type","expense"), Eq("from", myID))` — owner filter is implicit
  - Happy path: `report list --live --owner myk@esperlabs.ai` resolves the email to an accountID via the `people` table (Unit 4), builds filter `Eq("from", mykID)`, returns only Myk's reports
  - Happy path: `report list --live --all-visible` omits the owner filter and returns every visible report
  - Happy path: `--live` fetch upserts returned rows into the local store; subsequent non-live `expense list` includes them
  - Edge case: `ExpensifyAccountID` not configured AND `--all-visible` not set → error message "Run `expensify-pp-cli sync` first to identify your account" (exit 10, config error)
  - Edge case: `--owner <email>` not found in `people` table → error message suggesting `sync` to refresh the cache
  - Error path: `/Search` returns jsonCode 401 → exit 5 (API error) with the server's message shown
  - Error path: session expired (jsonCode 407) → exit 4 (auth), suggest `auth login`

  **Verification:**
  - With a valid session and `ExpensifyAccountID` set, `report list --live` returns only the user's own reports by default
  - `report list --live --all-visible` returns reports from all owners
  - `--live` results appear in subsequent local-only `report list` calls
  - Typed exit codes match the documented table (4/5/10)

- [ ] **Unit 3: `expense quick` merchant parser hardening**

  **Goal:** "Tacos $5" no longer sets `merchant="Tacos"`. The parser prefers explicit merchant cues ("at X") and refuses category-shaped leading nouns.

  **Requirements:** R5

  **Dependencies:** none

  **Files:**
  - Modify: `library/productivity/expensify/internal/cli/expense_quick.go`
  - Test: `library/productivity/expensify/internal/cli/expense_quick_test.go`

  **Approach:**
  - Introduce a small static `categoryNounBlocklist` of ~15 entries (tacos, lunch, dinner, coffee, ride, gas, etc.)
  - In the merchant-extraction path: if an "at X" phrase is present, keep the existing behavior (X is merchant). If no such phrase AND the leading word is in the blocklist, set merchant to empty and surface a hint
  - When merchant is empty AFTER parsing (and `--merchant` not provided), behavior depends on `--no-input`:
    - Interactive: prompt "No merchant detected from '<prompt>'. Enter merchant (or re-run with --merchant):"
    - `--no-input`: fail with exit 2 (usage) and a clear message naming the blocklist hit
  - `--merchant` always wins over parsed merchant

  **Patterns to follow:**
  - Existing `quickParsed` struct and the current regex-driven extractor in `expense_quick.go`
  - `--no-input` gate pattern used elsewhere in the CLI (helpers in `root.go`)

  **Test scenarios:**
  - Happy path: `"Dinner at Maya $42.50"` → merchant="Maya" (preserved existing behavior; "at X" wins over blocklist)
  - Happy path: `"Uber $24"` → merchant="Uber" (not in blocklist, leading noun stays)
  - Edge case: `"Tacos $5"` with `--no-input` → exit 2 with message "could not infer merchant from 'Tacos $5' — pass --merchant or include 'at <place>'"
  - Edge case: `"Tacos $5" --merchant "El Gordo"` → merchant="El Gordo", no parse error
  - Edge case: `"coffee $4"` → blocklist hit, same error path as tacos case
  - Edge case: empty prompt `""` → exit 2 (usage) before blocklist logic runs
  - Happy path: `"Lunch meeting with client at Panera $18"` → merchant="Panera" (at-X wins even though "Lunch" is blocklisted)

  **Verification:**
  - Every category-noun prompt without `--merchant` returns a clear error (not a silent merchant=category)
  - `--merchant` override always wins
  - Prompts with "at X" continue to work as before

- [ ] **Unit 4: `people` SQLite table + personalDetailsList sync**

  **Goal:** Store an accountID → displayName/login mapping in SQLite, synced from `/ReconnectApp`'s `personalDetailsList`. Expose a lookup helper the list commands use to render names.

  **Requirements:** R3

  **Dependencies:** none

  **Files:**
  - Modify: `library/productivity/expensify/internal/store/store.go` (add `people` table to `Migrate()`, add `Person` type, add `UpsertPerson`, `ListPeople`, `GetPersonByAccountID(id int64)`, `GetPersonByLogin(login string)`)
  - Modify: `library/productivity/expensify/internal/cli/sync.go` (extend `ingestReconnectApp` to ingest `personalDetailsList` entries)
  - Modify: `library/productivity/expensify/internal/config/config.go` (persist `ExpensifyAccountID` discovered from the ReconnectApp session object)
  - Test: `library/productivity/expensify/internal/store/store_test.go`

  **Approach:**
  - `people` schema: `account_id INTEGER PRIMARY KEY, display_name TEXT, login TEXT, avatar TEXT, synced_at TEXT`
  - `Person` struct mirrors the schema
  - Sync pass walks `onyxData[].value` when `key=="personalDetailsList"` — it's a map of stringified accountIDs to `{displayName, login, avatar}`
  - Also capture `session.accountID` from ReconnectApp and write it to config if unset (`ExpensifyAccountID` field) — used by Unit 2 for owner filtering
  - Sync returns `(nReports, nExpenses, nWorkspaces, nPeople)` — extend the final `Synced N reports, M expenses, K workspaces, P people` message

  **Patterns to follow:**
  - `UpsertWorkspace` for the upsert SQL shape with `ON CONFLICT`
  - Existing `walkMaps` in `sync.go` for map traversal
  - Existing config persistence in `config.go` (`save()` method)

  **Test scenarios:**
  - Happy path: `UpsertPerson({AccountID:20647491, DisplayName:"Myk Melez", Login:"myk@esperlabs.ai"})` inserts a new row; a second upsert with the same accountID and a different display name updates in place
  - Happy path: `GetPersonByAccountID(20647491)` returns the inserted row; `GetPersonByLogin("myk@esperlabs.ai")` returns the same row
  - Edge case: `GetPersonByAccountID(999)` for an unknown ID returns `(nil, sql.ErrNoRows)` — Unit 2 translates this to "accountID:999" fallback display
  - Edge case: `personalDetailsList` entry with missing `displayName` (some users have login only) upserts with empty display name, login still populated
  - Integration: a canned ReconnectApp response with 16 `personalDetailsList` entries upserts 16 rows AND writes the `session.accountID` to config when the field is empty
  - Integration: re-running sync after a displayName change in Expensify updates the cached row (upsert-on-conflict)

  **Verification:**
  - `expensify-pp-cli sync` after Unit 4 ships reports `"Synced ... P people from Expensify"` with P > 0
  - Config file gets an `expensify_account_id` entry after the first successful sync if not already present
  - Running `report list --live` (after Unit 2 ships) shows names like "Myk Melez" instead of raw account IDs

- [ ] **Unit 5: Sync walker issues `/Search` for full report history + filters task reports**

  **Goal:** `sync` pulls 12 months of historical reports via Search (not just the ReconnectApp summary) and upserts only real expense reports — no chats, no tasks, no onboarding reports.

  **Requirements:** R4

  **Dependencies:** Unit 1 (needs the Search helper), Unit 4 (needs the `people` table for owner lookups)

  **Files:**
  - Modify: `library/productivity/expensify/internal/cli/sync.go`
  - Test: `library/productivity/expensify/internal/cli/sync_test.go`

  **Approach:**
  - After the existing `ReconnectApp` ingest (which handles workspaces, categories, tags, current-snapshot reports, and personalDetailsList), issue a follow-up `Search` with `Query{Type:"expense-report", Filters:Eq("date","last-12-months"), SortBy:"date", SortOrder:"desc"}` to pull the full history
  - Parse the returned onyxData snapshot entries, walk `data.report_*` keys, upsert via `UpsertReport`
  - Filter each entry: upsert only when `type == "iou"` OR `type == "expenseReport"` OR `type == "expense-report"` (Expensify uses both spellings in different contexts). Skip `type == "chat"`, `type == "task"`, `type == "policyExpenseChat"`
  - Continue to upsert the 1-2 current reports that come back in the ReconnectApp summary (already working) — the Search pass adds to it, not replaces
  - Extend `sync` flags: `--since YYYY-MM-DD` already exists; add `--history-months N` (default 12, cap at 24) controlling the Search window

  **Patterns to follow:**
  - Existing `ingestReconnectApp` flow
  - `walkMaps` for onyxData traversal

  **Test scenarios:**
  - Happy path: a canned Search response with 26 `report_*` entries (mix of types) upserts only the 14 with `type=="iou"` or `type=="expense-report"`
  - Happy path: a canned response with 0 reports (new user) returns nReports=0, no error
  - Edge case: an entry with `type==""` (missing) defaults to skip (conservative)
  - Edge case: `--history-months 6` generates a Search with `Eq("date","last-6-months")`; `--history-months 36` clamps to 24 with a warning to stderr
  - Error path: Search returns jsonCode 401 → sync reports "historical fetch failed (invalid query)" to stderr, still commits the ReconnectApp results; exit 0 (partial sync is useful)
  - Error path: Search returns jsonCode 407 (session expired) → exit 4, don't write the partial ReconnectApp results (probably also stale)
  - Integration: end-to-end sync against a canned two-call response (ReconnectApp + Search) populates workspaces, people, current-reports (from ReconnectApp), AND historical-reports (from Search) without duplicates

  **Verification:**
  - `sync` output message includes historical report count: `"Synced 26 reports (12 historical), 4 expenses, 4 workspaces, 16 people"`
  - Paid reports from more than 1 month ago appear in the local `reports` table
  - `damage --month 2025-12` (after Unit 6 ships) returns non-zero PAID totals when the user had paid reports in that month

- [ ] **Unit 6: `damage` status bucketing uses mapped stateNum**

  **Goal:** Damage's Expensed / Pending / Approved / Paid buckets reflect real report state, not the current all-$0 output.

  **Requirements:** R6

  **Dependencies:** Unit 5 (needs real historical data in the cache to demonstrate non-zero buckets)

  **Files:**
  - Modify: `library/productivity/expensify/internal/store/store.go` (rewrite `Damage()` to bucket by mapped stateNum pulled from raw_json)
  - Modify: `library/productivity/expensify/internal/cli/damage.go` (no logic change; may need to surface a hint when all buckets still show $0 post-fix, e.g., "No reports synced for <month>. Run sync.")
  - Test: `library/productivity/expensify/internal/store/store_test.go`

  **Approach:**
  - `Damage()` currently groups by the `status` string column. Change it to parse `stateNum` from each report's `raw_json` field (already stored) and bucket with this mapping:
    - stateNum 0 → Expensed (open, not yet submitted)
    - stateNum 1 → Pending (submitted, awaiting approval)
    - stateNum 2 or 3 → Approved (approved but not yet reimbursed)
    - stateNum 4, 5, or 6 → Paid (reimbursed)
  - Also count "missing receipts" by querying expenses with `receipt == ''` for the month (existing logic, preserved)
  - Prefer populating a typed `stateNum` column on the reports table during Unit 5 sync so Damage doesn't parse raw_json at query time — but keep a fallback parse for rows synced before Unit 5

  **Patterns to follow:**
  - Existing `Damage()` method signature and return type (`StatusBreakdown` struct)
  - Existing `StatusBreakdown` struct fields: `Expensed`, `Pending`, `Approved`, `Paid`, `MissingReceipts`

  **Test scenarios:**
  - Happy path: 4 reports with stateNums [0, 1, 3, 4] in the target month sum into each of the 4 buckets
  - Happy path: no reports for the month → all $0 buckets with a stderr hint "No reports synced for <month>"
  - Edge case: a report with missing/invalid raw_json and no mapped stateNum column → falls through to "Expensed" as the safest default; count is reported separately
  - Edge case: stateNum=5 (BILLING — in transit) is treated as Paid (close enough for user-facing totals)
  - Integration: sync a 12-month history (Unit 5), then `damage --month 2025-12` shows the right totals per bucket; `damage --month 2030-01` (future) shows $0 with the sync hint

  **Verification:**
  - User's real $14,339 April 2026 paid report, after a fresh sync, shows up in `damage --month 2026-04`'s "Paid" bucket
  - Current all-$0 output no longer appears when the store has real data

- [ ] **Unit 7: `report consolidate` command**

  **Goal:** New `report consolidate` subcommand merges N draft reports into a single target report, optionally submits it, and leaves source drafts untouched.

  **Requirements:** R7

  **Dependencies:** Unit 2 (uses the updated report list and store methods)

  **Files:**
  - Create: `library/productivity/expensify/internal/cli/report_consolidate.go`
  - Modify: `library/productivity/expensify/internal/cli/report.go` (register the new subcommand)
  - Modify: `library/productivity/expensify/internal/store/store.go` (add `ListOpenReports(ownerAccountID int64, since time.Time) ([]Report, error)` and `ListExpensesInReport(reportID string) ([]Expense, error)` if not present)
  - Test: `library/productivity/expensify/internal/cli/report_consolidate_test.go`

  **Approach:**
  - Command signature: `report consolidate [--target <reportID>] [--since YYYY-MM-DD] [--title "..."] [--policy <id>] [--submit] [--dry-run]`
  - Flow:
    1. Read open drafts (stateNum=0) owned by the session user, optionally filtered by policy and/or date range
    2. If `--target` is set, use that report as the destination; otherwise pick the oldest draft as the destination (or create a new one if `--title` is given)
    3. Collect all expenses attached to the source drafts (using `ListExpensesInReport` for each)
    4. Emit a dry-run preview: "Will merge <N> drafts totaling $X into <target>: [draft titles]. Will attach <M> expenses."
    5. If not `--dry-run`: call `/AddExpensesToReport` with the collected expense transactionIDs and the target report ID
    6. If `--submit`: call `/SubmitReport` on the target after attachment
    7. Never delete source drafts; note in output that they're left empty ("Empty drafts can be deleted via `report delete <id>` if desired.")
  - Use existing client methods for `/AddExpensesToReport` and `/SubmitReport` — no new write-path request bodies
  - If any drafts are owned by different policies, refuse and list them: "Cannot merge drafts across different workspaces: [list]. Use --policy <id> to scope."

  **Patterns to follow:**
  - `library/productivity/expensify/internal/cli/report_draft.go` — its orchestration pattern (create → attach → optional submit) maps 1:1 to consolidate (pick/create → attach → optional submit)
  - `--dry-run` preview formatting

  **Test scenarios:**
  - Happy path: 6 open drafts in the default workspace, no `--target`, `--title "April consolidated"` → picks the oldest as target, renames via comment, attaches all expenses from the other 5
  - Happy path: `--target <reportID> --submit` → attaches, then submits; returns the final reportID and a "submitted" confirmation
  - Happy path: `--dry-run` prints the plan and exits 0 without calling any write endpoint
  - Edge case: only 1 open draft → exits 0 with message "Only 1 draft found. Nothing to consolidate."
  - Edge case: 0 open drafts → same as above
  - Edge case: drafts span two policies → exits with the cross-workspace refusal message, exit 2 (usage)
  - Edge case: `--target` references a report that isn't stateNum=0 → exit 2 with "Target report is not a draft (state=APPROVED)"
  - Error path: `/AddExpensesToReport` returns jsonCode 401 for one batch → report partial success (N of M expenses attached), exit 5
  - Error path: `--submit` fails after successful attach → exit 5 but stdout reports the attached count and suggests "Target report id=X has all expenses; submit manually with `report submit --report-id X`"

  **Verification:**
  - Dry-run preview shows correct counts against the user's live 6-draft state without calling any write endpoint
  - Non-dry-run consolidation attaches expected expenses and optionally submits
  - Cross-workspace and edge cases return usage errors with clear messages

## System-Wide Impact

- **Interaction graph:**
  - `client.Search()` is a new helper invoked by Units 2, 5, and potentially 7's preview step. It sits alongside the existing `Post` / `Get` / etc. wrappers — no refactor of the existing routing code
  - `sync.ingestReconnectApp` grows one new concern (personalDetailsList) and spawns one follow-up Search call (Unit 5). Sync becomes a slightly slower operation (~2 requests vs 1) but still under 2 seconds total
  - `people` table is a new dependency for the list commands (Unit 2) and for `--owner <email>` resolution
- **Error propagation:**
  - Search's typed errors (jsonCode 401 "Invalid Query", jsonCode 407 "Session expired") flow up through the existing `classifyAPIError` helper in `internal/cli/helpers.go`, which already maps them to the typed exit-code system (4=auth, 5=API, 10=config)
  - `sync` treats the Search pass as optional enrichment: if Search fails, the ReconnectApp results are still committed and a warning is printed to stderr
- **State lifecycle risks:**
  - Partial sync is acceptable and intended: if Search fails mid-flight, the store has current-snapshot data (from ReconnectApp) but not historical. Users re-run sync after fixing the failure
  - `people` cache can go stale (employees join/leave); not a correctness concern since lookups gracefully fall back to `accountID:N` display
  - `report consolidate` has a partial-attach risk: if `/AddExpensesToReport` fails mid-batch, N of M expenses landed. Test scenarios cover this; the CLI reports the partial state rather than silently succeeding
- **API surface parity:**
  - No other interface (MCP, webhook, etc.) is in scope — MCP is still a stub per non-goals. When the MCP bridge unstubs in a future plan, it will expose `search`, `consolidate`, and the upgraded list commands automatically via the same cobra tree
- **Integration coverage:**
  - Unit 5's end-to-end sync test exercises the ReconnectApp → Search → upsert → people-resolve chain with canned responses — this is where mocks-only tests fail and a full chain integration test catches real shape bugs
  - Unit 6's damage test needs Unit 5 data to prove non-zero buckets
  - Unit 2's list test needs Unit 4's people cache to prove name rendering
- **Unchanged invariants:**
  - `RequestMoney` / `CreateReport` / `SubmitReport` / `AddExpensesToReport` / `PayMoneyRequest` request body shapes — explicitly preserved
  - Existing `--dry-run`, `--json`, `--agent`, `--no-input` flags — no semantic change
  - `expense search` / `expense missing-receipts` / `expense rollup` / `expense dupes` — untouched (all work against local store; they benefit from Unit 5's richer data automatically)

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| `/Search` DSL drifts after Expensify changes it | The session interceptor pattern remains valid; re-capture is cheap. Tests use canned responses, so client-side changes land quickly |
| `hash` / `recentSearchHash` values the server rejects | Deferred to implementation; arbitrary positive integers work today per the evidence captured |
| `ownerAccountID` filter misses the user's own reports when `accountID` is stored as string vs number | Unit 4's test coverage includes both types; normalize to int64 in the store layer |
| Category-noun blocklist is too aggressive (blocks legitimate merchants) | Keep the list small (~15 words); `--merchant` always overrides. If feedback surfaces false positives (e.g., "Coffee" the merchant), expand the blocklist with a confidence threshold rather than dropping entries |
| `report consolidate` leaves empty drafts the user doesn't want | Output explicitly tells the user how to delete them; `--delete-empty-sources` flag can ship in a follow-up if requested |
| Sync performance regresses with the extra Search call | Search is bounded to 12 months and one round-trip; end-to-end sync stays under 2s on typical accounts. Add a `--no-history` flag if performance becomes a complaint |
| Partial sync of historical reports creates inconsistent `damage` totals | Damage query uses raw_json stateNum as the source of truth with the `reports.state_num` column as a fast-path; both converge to the same answer |

## Documentation / Operational Notes

- Update `library/productivity/expensify/README.md` Quick Start to mention `--live` on list commands and the new `report consolidate` verb
- Update `library/productivity/expensify/SKILL.md` Unique Capabilities with a new entry for `report consolidate`, and update the existing `report submit --wait` example to show the consolidate → submit chain as a common workflow
- No monitoring / rollout concerns — CLI changes are local-first and shipped via PR → release
- PR #104 description should note the fixes and reference this plan

## Sources & References

- **Session transcript (live dogfood):** captured jsonQuery DSL, personalDetailsList shape, policy employeeList shape, stateNum/statusNum enum values, owner-filter bug reproduction with 26 reports across 6 owners
- Related code: `library/productivity/expensify/internal/cli/sync.go` (ReconnectApp walker), `library/productivity/expensify/internal/store/store.go` (typed upsert helpers), `library/productivity/expensify/internal/client/client.go` (form-body request routing)
- Related PR: #104 — initial expensify-pp-cli submission
- Upstream spec: the synthetic YAML at `~/printing-press/.runstate/mvanhorn-eb6a05f2/runs/20260420-121903/research/expensify-spec.yaml` (generator input; not modified by this plan)
