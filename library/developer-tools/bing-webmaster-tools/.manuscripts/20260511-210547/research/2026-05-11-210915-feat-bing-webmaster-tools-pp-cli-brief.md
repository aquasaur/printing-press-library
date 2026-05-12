# Bing Webmaster Tools CLI Brief

## API Identity
- Domain: SEO / search-engine webmaster management for Bing (and partners that consume Bing's index).
- Users: SEO managers, technical SEOs, agencies managing many sites, site owners monitoring Bing visibility, and increasingly AI agents doing automated SEO ops.
- Data profile: per-site time series (clicks/impressions/CTR/position by day), per-query and per-page stats, keyword research data, crawl issues, inbound link counts, sitemaps/feeds, URL/content submission quotas, and a pile of per-site config (crawl rate, blocked URLs, page-preview blocks, deep-link blocks, query-parameter normalization, country/region targeting, site roles).

## API Shape
- Base URL: `https://ssl.bing.com/webmaster/api.svc/json/<MethodName>`
- Auth: API key as query param `?apikey=<KEY>` (OAuth also supported; API key is the documented easy path). Env var: `BING_WEBMASTER_API_KEY`.
- Format: JSON. Reads = `GET` with query params; writes = `POST` with a JSON request body. Responses are wrapped in a `{"d": ...}` envelope (WCF .svc style).
- Quirks: dates serialized as `/Date(ms)/`; `{"d": ...}` wrapping; tight daily quota on `SubmitUrl` (often ~10/day for new accounts — Microsoft now steers people to IndexNow for real-time submission); no official OpenAPI spec — the canonical surface is the `.NET` `IWebmasterApi` reference plus community reimplementations.

## Reachability Risk
- None. `GET https://ssl.bing.com/webmaster/api.svc/json/GetUserSites?apikey=TEST` returns `400 {"ErrorCode":3,"Message":"ERROR!!! InvalidApiKey"}` — the API processes programmatic requests and returns structured errors. A real key will work for live smoke testing.

## API Surface (the full method catalog to absorb)
Cross-referenced from the MS Learn `IWebmasterApi` reference, the `isiahw1/mcp-server-bing-webmaster` MCP server (~40 tools, 11 categories), the `merj/bing-webmaster-tools` Python client, the `seo-meow` Rust client, and the `NmadeleiDev/bing_webmaster_cli` Go CLI.

- **Sites:** GetUserSites, AddSite, VerifySite, RemoveSite, GetSiteRoles, AddSiteRoles, RemoveSiteRole, GetChildSitesInfo, AddChildSite, GetSiteMoves / SubmitSiteMove
- **Sitemaps & feeds:** GetFeeds, SubmitFeed, RemoveFeed
- **URL submission:** SubmitUrl, SubmitUrlBatch, GetUrlSubmissionQuota, GetUrlSubmissionQuotaByDate?, GetFetchedUrls, GetFetchedUrlDetails, FetchUrl
- **Content submission:** SubmitContent, GetContentSubmissionQuota
- **Traffic & index analytics:** GetRankAndTrafficStats, GetQueryStats, GetPageStats, GetQueryPageStats, GetQueryPageDetailStats, GetUrlTrafficInfo, GetUrlInfo
- **Crawl control:** GetCrawlStats, GetCrawlIssues, GetCrawlSettings, SaveCrawlSettings
- **Links:** GetLinkCounts, GetUrlLinks, GetConnectedPages, AddConnectedPage, RemoveConnectedPage
- **Keyword research:** GetKeyword, GetRelatedKeywords, GetKeywordStats
- **Content blocking:** GetBlockedUrls, AddBlockedUrl, RemoveBlockedUrl
- **Page preview blocks:** GetActivePagePreviewBlocks, AddPagePreviewBlock, RemovePagePreviewBlock
- **Deep link blocks:** GetDeepLinkBlocks, AddDeepLinkBlock, RemoveDeepLinkBlock, GetDeepLinkAlgoUrl?
- **Query-parameter normalization:** GetQueryParameters, AddQueryParameter, RemoveQueryParameter
- **Geo targeting:** GetCountryRegionSettings, AddCountryRegionSettings, RemoveCountryRegionSettings

That is ~50 methods. The competitor CLI covers ~6.

## Top Workflows
1. **Site health snapshot** — list sites, pull rank/traffic stats, crawl issues, link counts, URL-submission quota for a site in one go.
2. **Search-performance analysis** — query stats + page stats + query-page stats over a date range; find which queries drive clicks vs impressions; sort by CTR or position.
3. **Index management** — check whether URLs are indexed (`GetUrlInfo`), submit URLs / batches for indexing, watch the daily quota, submit/remove sitemaps.
4. **Keyword research** — keyword data + related keywords + historical keyword stats for content planning.
5. **Crawl & config hygiene** — review crawl issues, adjust crawl rate, manage blocked URLs / query-parameter normalization / geo targeting / page-preview & deep-link blocks.
6. **Multi-site agency ops** — run the same pull across every site in the account and diff over time.

## Table Stakes (must match the competitor CLI + community libs)
- List sites; site + URL traffic stats with date-range filtering; URL index check with crawl-issue reason hints when not indexed; submit single / batch URL; output `table` / `json` / `csv`; API-key auth from `BING_WEBMASTER_API_KEY`.
- Full coverage of the documented method catalog above (the community Python/Rust/PHP clients each expose the whole surface — so must we).

## Data Layer
- Primary entities: `sites`, `feeds` (sitemaps), `rank_traffic_stats` (daily series per site), `query_stats`, `page_stats`, `query_page_stats`, `keyword_stats`, `crawl_issues`, `link_counts`, `url_links`, `blocked_urls`, `submitted_urls` (with submit timestamps + quota snapshots), `crawl_settings`, `country_region_settings`.
- Sync cursor: date-based — most stats endpoints accept a date range; sync pulls the trailing window per site and upserts daily rows. Site list and config tables sync as full snapshots.
- FTS/search: `query_stats.query`, `page_stats.page`, `keyword_stats.keyword`, `crawl_issues.url` — so `search "checkout"` finds matching queries/pages/issues across the synced store.

## Why install this instead of the incumbent
- The competitor CLI (`bing_webmaster_cli`) does ~6 things. This CLI absorbs the **entire** documented API surface, then adds: a local SQLite store + FTS offline search; agent-native output on every command (`--json`, `--select` dotted paths, `--csv`, `--dry-run`, typed exit codes); and transcendence commands that only work because everything is local — traffic-trend analysis over time, query/page CTR-opportunity finder (high impressions + low CTR), keyword-cannibalization detection (multiple pages ranking for one query), crawl-issue triage, and quota-aware batch submission.
- Agent-native MCP surface comes for free (the runtime mirrors the Cobra tree), so any agent can drive Bing SEO ops.

## Product Thesis
- Name: `bing-webmaster-tools-pp-cli` (slug `bing-webmaster-tools`).
- Why it should exist: Bing Webmaster Tools has no official spec and no first-party CLI; the existing tooling is a scattering of language-specific wrappers, one thin CLI, and an MCP server. A single agent-native Go CLI that covers the whole API, persists data locally, and adds SEO analytics on top is strictly better for both humans running SEO ops and agents automating them.

## Discovery gates
- Browser-sniff gate: **declined** — at the Phase 0 disambiguation the user was offered "the dashboard website itself" (which would mean browser-sniffing the logged-in dashboard's internal endpoints) vs "the official documented API" and chose the documented API. The documented `ssl.bing.com/webmaster/api.svc` surface is what we generate from.
- Crowd-sniff: not run — the API surface is fully enumerated from the MS Learn `IWebmasterApi` reference plus four community client libraries and an MCP server; npm/GitHub mining would only re-confirm known methods.

## Build Priorities
1. Internal YAML spec covering the full method catalog (reads as GET+query, writes as POST+body, `apikey` query auth) + `{"d": ...}` envelope handling + `/Date()/` parsing.
2. Data layer + sync/search/SQL for the stats and config entities.
3. All absorbed methods as commands.
4. Transcendence: traffic trends, CTR-opportunity finder, keyword cannibalization, crawl-issue triage, quota-aware batch submit, site-health snapshot.
