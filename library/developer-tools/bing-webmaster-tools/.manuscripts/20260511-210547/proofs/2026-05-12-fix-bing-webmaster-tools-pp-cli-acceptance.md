# Live Acceptance — bing-webmaster-tools-pp-cli

**Level:** Quick (read-only matrix + one dry-run; no live writes against the real account).
**Credential:** `BING_WEBMASTER_API_KEY` supplied by the operator, used read-only for Phase 5 only.
**Account:** the operator's Bing Webmaster Tools account, 3 verified sites (a WordPress blog plus two of the operator's own domains).

## Tests: 11 / 11 passed

| # | Check | Result |
|---|---|---|
| 1 | `doctor --json` | PASS — `auth: configured`, `credentials: valid`, `api: reachable`, env vars 1/1. |
| 2 | `sites list --json` | PASS — returns 3 verified sites. Response was already unwrapped from Bing's `{"d":…}` envelope (the client hand-edit works). |
| 3 | `submit quota --site-url <site> --json` | PASS — `DailyQuota: 100, MonthlyQuota: 2000`. |
| 4 | `sites health --site-url <site> --json` | PASS — `dataComplete: true`, score 93/A, real metrics (9 clicks / 209 impressions, 1194 crawled pages, 2223 in index, 81 crawl errors, blocked-by-robots 352, quota 100/2000). |
| 5 | `sites triage --json` | PASS — ranks the busiest site first (81 crawl errors), the two quiet sites after; one row per site with crawl-issue count, crawl errors, clicks, impressions. |
| 6 | `url check https://<site>/ --json` | PASS — `indexed: true`, `lastCrawledDate` and `discoveryDate` populated and normalized to ISO-8601 (the `/Date()/` hand-edit works), `documentSize` realistic, reason "URL appears indexed by Bing; no crawl issues recorded." |
| 7 | `crawl triage --site-url <site> --json` | PASS — returns `[]` for a site Bing reports zero per-URL crawl issues for (correct: `GetCrawlIssues` was empty; the `crawlErrors` count in crawl *stats* is a separate aggregate). |
| 8 | `keywords cannibalization --site-url <site> --json` | PASS — returns `[]` for a site where no query has 2+ ranking pages (correct empty result, not a fabricated one). |
| 9 | `traffic queries --site-url <site> --json --select Query,Clicks,Impressions` | PASS — 119 query rows, field selection narrows to the 3 requested keys. (Note: a `sync`-side warning fires — the store has no extractable ID field for stat rows; analytics commands work live regardless. Logged as a known limitation.) |
| 10 | `traffic ctr-gaps --site-url <site> --min-impressions 1 --json` | **FIXED then PASS.** Initially returned `[]` because the `int(lostClicks)==0` filter discarded every low-impression query. Fixed (`internal/cli/traffic_ctr_gaps.go`): `estLostClicks` is now a float, the `lost <= 0` filter was removed (the `ctr < expectedCtr` check already guarantees a real gap), and rows sort by the float. Re-run: returns queries ranked by estimated lost clicks ("ai coaching platform" 0.3, "team map" 0.25, …). |
| 11 | `submit smart --site-url <site> --dry-run --json` (URLs piped on stdin) | PASS — reports the submission plan with `dryRun: true`; never calls the API. |

## Fixes applied this phase
- `internal/cli/traffic_ctr_gaps.go` — `estLostClicks` int→float; dropped the over-aggressive `lost <= 0` filter so low-traffic sites still surface their CTR gaps. Synced into the promoted library copy and the library binary was rebuilt.

## Printing Press issues observed (retro candidates)
- `printing-press lock promote` panics (`panic: assignment to entry in nil map` in `pipeline.LoadState` at `state.go:510`, via `NewMinimalState`) when re-run after a prior successful promote — the first promote of this CLI succeeded; a second one panicked. Worked around by syncing the one changed file into the library dir and rebuilding the binary directly.
- The internal-spec store can't derive an ID field for response types whose identifier is `Url` (not `Id`/`id`), so `sync` skips every row of the stats tables and `search`/`sql` stay empty for them. (Compounds with the already-documented "`sync` doesn't fan a required `siteUrl` param" limitation.) An `id_field:` hint on internal-spec types would close this.

## Gate: PASS
All 11 checks pass (one after an in-session fix). Auth and the API are reachable. No live writes were performed. The CLI is promoted to `~/printing-press/library/bing-webmaster-tools/` with the fix included.
