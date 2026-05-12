# Absorb Manifest — bing-webmaster-tools-pp-cli

## Ecosystem scanned
- `isiahw1/mcp-server-bing-webmaster` — MCP server, ~40 tools across 11 categories (the most complete feature catalog of the live API).
- `merj/bing-webmaster-tools` — Python async client, full surface, service-organized.
- `seo-meow/bing-webmaster-api` — Rust client, "all methods from Microsoft's Bing Webmaster Tools".
- `webjeyros/bing-webmaster-api` — PHP client.
- `charlesnagy/bing-webmastertools-python` — Python client.
- `NmadeleiDev/bing_webmaster_cli` — the direct competitor: a Go CLI, "built for coding agents/LLMs", ~6 commands.
- `webmaster-api` (npm), `ping_bing` (npm) — older/partial Node wrappers.
- `analyticsedge.com` Bing Webmaster API blog — Excel-based, documents endpoint quirks (`/Date()/` formats, the `{"d"}` envelope).
- Microsoft Learn `IWebmasterApi` reference — canonical method list.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List sites in account | bing_webmaster_cli `site list` / merj `sites.get_sites` | `sites list` (GetUserSites) | Cached in store; `--json`/`--csv`/`--select` |
| 2 | Add / verify / remove site | mcp `add_site`/`verify_site`/`remove_site` | `sites add`, `sites verify`, `sites remove` | `--dry-run`, typed errors |
| 3 | Site roles list / delegate / revoke | mcp `get_site_roles`/`add_site_roles` | `sites roles list/add/remove` (GetSiteRoles/AddSiteRoles/RemoveSiteRole) | `--dry-run` |
| 4 | Child sites / site moves | merj `sites` | `sites children`, `sites moves`, `sites submit-move` (GetChildSitesInfo/AddChildSite/GetSiteMoves/SubmitSiteMove) | structured output |
| 5 | Get / submit / remove sitemaps & feeds | mcp `get_feeds`/`submit_sitemap`/`remove_sitemap` | `feeds list/submit/remove` (GetFeeds/SubmitFeed/RemoveFeed) | cached, `--dry-run` |
| 6 | Submit single URL for indexing | mcp `submit_url` / Bing cURL blog | `submit url <url>` (SubmitUrl) | `--dry-run`; records to store with timestamp + re-submit warning |
| 7 | Submit batch URLs | mcp `submit_url_batch` | `submit batch --file urls.txt` (SubmitUrlBatch) | reads stdin/file, `--dry-run` |
| 8 | URL submission quota | mcp `get_url_submission_quota` | `submit quota` (GetUrlSubmissionQuota) | cached snapshot |
| 9 | Submit page content directly | mcp `submit_content` | `submit content <url> --html FILE` (SubmitContent) | `--dry-run` |
| 10 | Content submission quota | mcp `get_content_submission_quota` | `submit content-quota` (GetContentSubmissionQuota) | cached |
| 11 | Fetched URLs / fetch-as-Bingbot | merj submission | `fetch list`, `fetch get <id>`, `fetch url <url>` (GetFetchedUrls/GetFetchedUrlDetails/FetchUrl) | structured output |
| 12 | Rank & traffic stats (clicks/impressions by day) | bing_webmaster_cli traffic / mcp `get_rank_and_traffic_stats` | `traffic stats --site S` (GetRankAndTrafficStats) | synced into store as daily series; `--json`/`--csv` |
| 13 | Query stats (search query performance) | mcp `get_query_stats` | `traffic queries --site S` (GetQueryStats) | FTS-searchable in store |
| 14 | Page stats | mcp `get_page_stats` | `traffic pages --site S` (GetPageStats) | FTS-searchable |
| 15 | Query-page stats / detail | mcp `get_query_page_stats`/`get_query_page_detail_stats` | `traffic query-pages`, `traffic query-page-detail` (GetQueryPageStats/GetQueryPageDetailStats) | structured |
| 16 | URL traffic info / URL index info | mcp `get_url_traffic_info`/`get_url_info` | `url traffic <url>`, `url info <url>` (GetUrlTrafficInfo/GetUrlInfo) | structured |
| 17 | Crawl stats / bot activity | mcp `get_crawl_stats` | `crawl stats --site S` (GetCrawlStats) | synced daily series |
| 18 | Crawl issues / errors | bing_webmaster_cli (reason hints) / mcp `get_crawl_issues` | `crawl issues --site S` (GetCrawlIssues) | cached + FTS; bitflag decoding |
| 19 | Get / save crawl settings (rate) | mcp `get_crawl_settings`/`update_crawl_settings` | `crawl settings get/set --rate slow\|normal\|fast` (GetCrawlSettings/SaveCrawlSettings) | `--dry-run` |
| 20 | Inbound link counts | mcp `get_link_counts` | `links counts --site S` (GetLinkCounts) | cached |
| 21 | Inbound links for a URL | mcp `get_url_links` | `links url <url>` (GetUrlLinks) | structured |
| 22 | Connected pages list / add / remove | mcp `add_connected_page` / merj links | `links connected list/add/remove` (GetConnectedPages/AddConnectedPage/RemoveConnectedPage) | `--dry-run` |
| 23 | Keyword data | mcp `get_keyword_data` | `keywords data <kw>` (GetKeyword) | structured |
| 24 | Related keywords | mcp `get_related_keywords` | `keywords related <kw>` (GetRelatedKeywords) | `--json`/`--csv` |
| 25 | Keyword historical stats | mcp `get_keyword_stats` / analyticsedge | `keywords stats <kw>` (GetKeywordStats) | synced into store as series |
| 26 | Blocked URLs list / add / remove | mcp `get_blocked_urls`/`add_blocked_url`/`remove_blocked_url` | `block-urls list/add/remove` (GetBlockedUrls/AddBlockedUrl/RemoveBlockedUrl) | `--dry-run` |
| 27 | Page-preview blocks list / add / remove | mcp `get_active_page_preview_blocks`/… | `page-preview list/add/remove` (GetActivePagePreviewBlocks/AddPagePreviewBlock/RemovePagePreviewBlock) | `--dry-run` |
| 28 | Deep-link blocks list / add / remove | mcp `get_deep_link_blocks`/… | `deep-links list/add/remove` (GetDeepLinkBlocks/AddDeepLinkBlock/RemoveDeepLinkBlock) | `--dry-run` |
| 29 | Query-parameter normalization list / add / remove | mcp `get_query_parameters`/… | `query-params list/add/remove` (GetQueryParameters/AddQueryParameter/RemoveQueryParameter) | `--dry-run` |
| 30 | Country/region targeting get / add / remove | mcp `get_country_region_settings`/… | `geo list/add/remove` (GetCountryRegionSettings/AddCountryRegionSettings/RemoveCountryRegionSettings) | `--dry-run` |
| 31 | Output formats table/json/csv | bing_webmaster_cli | every command (`--json`, `--csv`, `--select` dotted paths, `--compact`) | dotted-path field selection |
| 32 | API-key auth from env | bing_webmaster_cli (`BING_WEBMASTER_API_KEY`) | config + `doctor` (checks key presence + API reachability) | doctor command, typed errors |
| 33 | URL index check with crawl-issue reason hints | bing_webmaster_cli `url check-index` | `url check <url>` — GetUrlInfo + GetCrawlIssues joined, decodes issue flags to plain-English reason | joins two endpoints, agent-native verdict |
| 34 | Offline search across synced data | (none — incumbents have no store) | `search "<term>"` (FTS over queries, pages, keywords, crawl-issue URLs) | works offline, regex, SQL-composable |
| 35 | Local SQL over synced data | (none) | `sql "<SELECT …>"` | composable analytics |
| 36 | Sync the stats time series | (none) | `sync --site S [--days N]` upserts daily rows for rank/traffic, query, page, query-page, crawl, keyword stats; full snapshots for sites + config | makes every transcendence feature possible |

No row ships as a stub. Write-side commands (add/remove/submit/save) print by default and require the action flag is implied by the verb; `--dry-run` shows the request without sending; verify-env short-circuits.

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|-------------------------|---------|
| 1 | Site health snapshot | `site health <site>` | 8/10 | One scored summary joining GetRankAndTrafficStats + GetCrawlStats + GetCrawlIssues + GetLinkCounts + GetUrlSubmissionQuota — no single API call returns it; decodes crawl bitflags into the verdict. | Priya, Marcus |
| 2 | All-sites triage board | `site triage` | 8/10 | Ranks every verified site worst-first by crawl-issue count + traffic delta from the local synced store — there is no cross-site endpoint; only the SQLite store has the data side by side. | Marcus |
| 3 | CTR-opportunity finder | `traffic ctr-gaps <site> --min-impressions N` | 7/10 | Local SQL over synced `query_stats`/`page_stats` surfaces high-impression + low-CTR rows ranked by estimated lost clicks — the API returns raw stats, not the opportunity ranking. | Priya, Sana |
| 4 | Keyword cannibalization detector | `keywords cannibalization <site>` | 7/10 | Groups synced `query_page_stats` by query to find queries where 2+ of the site's own pages rank, ranked by split-impression cost — a local groupby no single endpoint performs. | Sana |
| 5 | Quota-aware batch submit | `submit smart --file urls.txt` | 8/10 | Reads GetUrlSubmissionQuota, submits only up to the remaining daily allowance via SubmitUrlBatch, records every submission to a local ledger with timestamp, and exits non-zero if any URL was dropped — the quota arithmetic + dropped-URL CI signal exist nowhere else. | Dev |
| 6 | Crawl-issue triage | `crawl triage <site>` | 7/10 | Decodes synced `crawl_issues` bitflags to plain-English categories, groups by category, and joins to `page_stats` so high-traffic broken pages float to the top — flag decoding + traffic weighting is net-new on top of GetCrawlIssues. | Priya, Marcus |

Killed candidates (`traffic trends`, `traffic query-movers`, `keywords plan`, `submit history`, `feeds health`, `url coverage`, `config audit`, `keywords footprint`, `links trends`) and their kill reasons are recorded in `2026-05-11-210915-novel-features-brainstorm.md`.
