# Build Log — bing-webmaster-tools-pp-cli

## What was built
- **Spec:** hand-authored internal YAML spec (`research/bing-webmaster-tools-spec.yaml`) covering ~56 documented Bing Webmaster Tools API methods across 13 resources (sites, feeds, submit, fetch, traffic, url, crawl, links, keywords, block-urls, page-preview, deep-links, query-params, geo). Auth: `api_key` in query param `apikey`, env `BING_WEBMASTER_API_KEY`. `mcp:` block opts into `transport: [stdio, http]` + `orchestration: code` + `endpoint_tools: hidden` (56 tools > 50 threshold).
- **Generated:** full CLI — all spec endpoints as commands, `sync`/`search`/`sql`/`workflow`/`import`, `doctor`, agent-native flags. All 8 quality gates pass.
- **Transcendence commands (Priority 2), hand-built:**
  - `sites health [site-url]` — joins GetRankAndTrafficStats + GetCrawlStats + GetCrawlIssues + GetLinkCounts + GetUrlSubmissionQuota into one scored report; decodes crawl bitflags into the verdict.
  - `sites triage` — walks GetUserSites and ranks every site worst-first by crawl-issue count + crawl errors + (inverse) clicks.
  - `traffic ctr-gaps [site-url] --min-impressions N` — high-impression / low-CTR queries ranked by estimated lost clicks (expected-CTR-by-position curve computed locally).
  - `keywords cannibalization [site-url]` — groups GetQueryPageStats by query, flags queries with 2+ ranking pages, ranks by total impressions.
  - `crawl triage [site-url]` — decodes crawl-issue bitflags into named categories, groups, joins to GetPageStats so high-traffic broken pages float to the top.
  - `submit smart [site-url] --file/--url/stdin` — reads GetUrlSubmissionQuota, submits only up to the remaining allowance via SubmitUrlBatch, records each submission to the local `submit` table with a timestamp, exits non-zero (typed exit 2) if any URL was dropped. `cliutil.IsVerifyEnv()` short-circuit + `--dry-run` aware.
  - `url check [url]` — (absorbed #33, the incumbent CLI's headline feature, matched & extended) joins GetUrlInfo + GetCrawlIssues for one URL, infers indexed/not, decodes crawl flags into a plain-English reason.
  - Wired via `internal/cli/novel.go`'s `attachNovelCommands`, called from `root.go` (one hand-edited line). Shared helpers in `internal/cli/novel_helpers.go`.

## Hand-edits to generated files (not regen-safe — documented)
- `internal/client/client.go` — extended `sanitizeJSONResponse` to (a) unwrap the WCF `{"d": <payload>}` response envelope every `.svc` method returns, and (b) normalize legacy `/Date(ms)/` date literals to RFC 3339. **Retro candidate:** internal specs have no `response_envelope_key` knob; without this hand-edit every command's `--json` output and the local store would carry the `{"d":}` wrapper.
- `internal/cli/root.go` — one line: `attachNovelCommands(rootCmd, flags)` before `return rootCmd`, to register the hand-built commands as children of the generated `sites`/`traffic`/`keywords`/`submit`/`crawl`/`url` parents.

## Intentionally deferred / known limitations
- **`sync` only populates account-wide tables.** Most stats endpoints (GetRankAndTrafficStats, GetQueryStats, GetCrawlStats, GetKeywordStats, …) require a `siteUrl` parameter, which `sync` does not supply, so `sync` populates `sites` (and `submit` via `submit smart`) but not the per-site stats tables. The transcendence analytics commands therefore work **live** rather than off the store. The narrative and SKILL/README troubleshooting reflect this honestly. **Retro candidate:** the generator's `sync` has no way to fan a required path/query param (like `siteUrl`) across a set of values, so required-param GET endpoints can't be synced.
- Crawl-issue bitflag → category mapping (`crawlIssueCategories`) is a best-effort decode of the documented `CrawlIssueType` flags enum; bits without a known name are reported as `flag_<bit>`. Exact bit values may drift; the grouping/traffic-weighting is the load-bearing part.
- `GetCountryRegionSettings` / `GetQueryParameters` are documented as possibly requiring elevated account permissions — they'll return a clear API permission error, not a stub.
- IndexNow (Microsoft's preferred real-time submission protocol, separate from `SubmitUrl`) is out of scope — it's a different endpoint family on `bing.com/indexnow`, not part of the Webmaster Tools API. Noted in the SKILL troubleshooting.

## Reachability
`GET https://ssl.bing.com/webmaster/api.svc/json/GetUserSites?apikey=TEST` → `400 {"ErrorCode":3,"Message":"ERROR!!! InvalidApiKey"}`. The API processes programmatic requests and returns structured errors. Live smoke testing pending a real `BING_WEBMASTER_API_KEY`.

## Generator notes (retro candidates)
1. No `response_envelope_key` / response-unwrap config for internal specs — WCF `.svc` JSON APIs (`{"d":}` envelope) need a hand-edit to `client.go`.
2. No legacy-date-format normalization hook — `/Date(ms)/` had to be regexed in the same hand-edit.
3. `sync` can't fan a required `siteUrl`-style param across values, so required-param GET endpoints don't sync.
4. No first-class extension point for registering hand-built novel commands — had to add one line to `root.go` (DO NOT EDIT) plus a `novel.go` shim.
