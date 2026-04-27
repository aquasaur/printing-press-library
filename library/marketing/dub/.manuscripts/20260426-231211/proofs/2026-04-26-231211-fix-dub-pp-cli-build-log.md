# Build Log — dub-pp-cli (run 20260426-231211)

## What was built

### Generated (Phase 2)
- 53 spec operations as commands across 14 resource groups (links, partners, analytics, tags, folders, domains, customers, commissions, bounties, payouts, track, qr, events, embed-tokens)
- Standard generator-emitted commands: `agent-context`, `api`, `auth`, `doctor`, `export`, `feedback`, `import`, `profile`, `search`, `sync`, `tail`, `which`, `workflow`, `analytics`
- Internal packages: `cliutil`, `client`, `config`, `store`, `types`, `mcp`, `cache`
- Local SQLite store with tables for all 14 resources, FTS indexes, sync_state tracking

### Hand-built transcendence (Phase 3)
Top-level commands:
- `campaigns` — tag-grouped performance dashboard
- `funnel` — click → lead → sale conversion attribution
- `health` — workspace health doctor (rate limit, expired-active links, dead destinations, dormant tags)
- `since [duration]` — time-windowed change feed

`links` subcommands:
- `links stale` — dead-link detection (zero-click, expired, archived-but-trafficked)
- `links drift` — week-over-week click-rate drop detection
- `links duplicates` — find links pointing to same destination
- `links lint` — slug-collision/typo/reserved-word audit
- `links rewrite` — bulk URL/UTM rewrite with diff/dry-run

`partners` subcommands:
- `partners leaderboard` — rank by earnings/conversion/clicks/leads/sales
- `partners audit-commissions` — reconcile partners × commissions × payouts

`customers` subcommands:
- `customers journey` — single-customer timeline across links/leads/sales

`domains` subcommands:
- `domains report` — per-domain link distribution + click share

Total: 13 transcendence features. All shipping-scope, no stubs.

### Tests added (per Phase 3 Completion Gate rule)
Pure-logic test files with table-driven assertions:
- `links_stale_test.go` — `classifyStale` covering reasons & threshold logic
- `links_drift_test.go` — `parseWindow`, `computeDriftPct` edge cases (0→N, N→0, both-zero)
- `links_duplicates_test.go` — `normalizeURL` UTM-strip behavior
- `links_lint_test.go` — `isLookalike`, `lintSlugs` reserved/case-collision/lookalike
- `campaigns_test.go` — `tagNamesFrom`, `sortCampaigns`
- `funnel_test.go` — `parseAnalyticsCount`, `pctOf`
- `partners_audit_commissions_test.go` — `isOlderThanDays`, `parseTimestamp`

`go test ./internal/cli/...` → PASS.

### Wiring
- `internal/cli/transcend.go` — `registerTranscendence(root, &flags)` grafts hand-built commands onto the generated cobra tree
- `internal/cli/root.go` — single line added to call `registerTranscendence` after generated AddCommands
- `internal/cli/timehelpers.go` — `nowFunc` + `parseTimestamp` shared across health/since/audit-commissions

## Generator warnings (non-blocking)
- `geo`, `testVariants`, `linkProps`, `metadata`, `partner`, `data` body fields skipped (complex types not supported as CLI flags). These would need first-class JSON-stdin support to ship as flags. Not flagged as blockers — `--data` JSON-stdin path covers them.

## Auth
- Generator emitted `DUB_TOKEN` (bearer_token convention → `_TOKEN`). Spec example was `DUB_API_KEY` but the generator's convention takes precedence. User's key works under either name; the CLI reads `DUB_TOKEN`.
- Doctor passes (Config OK, Auth configured, API reachable, Credentials valid).

## Quality gates (Phase 2 emission)
- PASS go mod tidy
- PASS go vet ./...
- PASS go build ./...
- PASS build runnable binary
- PASS dub-pp-cli --help
- PASS dub-pp-cli version
- PASS dub-pp-cli doctor

## Priority 1 Review Gate (Phase 3)
Sampled 3 random commands; all passed:
- `campaigns --help` — realistic Examples block ✓
- `links stale --help` — Examples block with --agent and --json ✓
- `funnel --help` — Examples block, flags documented ✓
- `--json` produces valid JSON envelope ✓

## Priority 2 deferred
None. All 13 transcendence features approved at Phase Gate 1.5 are built.
