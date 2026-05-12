# Novel Features Brainstorm — bing-webmaster-tools-pp-cli

## Customer model

**Priya — in-house technical SEO at a mid-size ecommerce brand (one big site, ~50k URLs).** Lives in Google Search Console; Bing is the monthly-guilt tab. Weekly: Monday pull of last week's query/page stats, hunt position losses, re-submit URLs that 404'd after a deploy — burns the SubmitUrl daily quota fast with no visibility. Frustration: Bing data trapped behind a clicky dashboard; quota is a black box; can't diff "queries bleeding clicks vs last month" without manual work.

**Marcus — freelance SEO consultant / small agency, 15–30 client sites.** Spreadsheet with a row per client, copy-pastes Bing numbers monthly. Owns `bing_webmaster_cli` but it does ~6 things. Weekly: Friday "is anything on fire" pass — but with 25 sites that's 25 logins so he mostly skips it. Frustration: no way to run one command across all sites and get a ranked worst-first list.

**Dev — platform engineer who owns the deploy pipeline; SEO is a side duty.** After a release, manually submits changed URLs through the dashboard, hits the quota, gives up. Per deploy he wants to feed a list of changed URLs to Bing, see what got accepted, and get a non-zero CI exit if it failed. Frustration: SubmitUrl ~10/day cap with no visibility; opaque batch behavior; no quota-respecting CLI.

**Sana — content strategist doing Bing/Microsoft-search keyword and content planning.** Uses the dashboard keyword tool one keyword at a time; guesses seasonality from a sparkline; no way to see which of her own pages already rank for a target query before briefing a new one. Weekly: research a batch of seed keywords, pull related terms, check historical volume, cross-reference against page performance. Frustration: keyword research and "what already ranks" live on two screens with no join — cannibalization caught only after publishing.

## Candidates (pre-cut)

(See survivors/kills below. 15 candidates generated across sources: persona-driven, service-specific patterns — quota, crawl-issue flags, query/page split, `{"d"}` envelope, IndexNow steerage — and cross-entity local joins over query_stats × page_stats × rank_traffic_stats × crawl_issues × link_counts × keyword_stats × submitted_urls.)

## Survivors

| # | Feature | Command | Score | Persona | Buildability |
|---|---------|---------|-------|---------|--------------|
| 1 | Site health snapshot | `site health <site>` | 8/10 | Priya, Marcus | Joins GetRankAndTrafficStats + GetCrawlStats + GetCrawlIssues + GetLinkCounts + GetUrlSubmissionQuota into one scored summary; spec endpoints only. |
| 2 | All-sites triage board | `site triage` | 8/10 | Marcus | Reads synced per-site `rank_traffic_stats` + `crawl_issues` for every site; ranks worst-first; pure local aggregation. |
| 3 | CTR-opportunity finder | `traffic ctr-gaps <site> --min-impressions N` | 7/10 | Priya, Sana | Local SQL over synced `query_stats`/`page_stats`: high-impression + low-CTR rows ranked by estimated lost clicks. |
| 4 | Keyword cannibalization detector | `keywords cannibalization <site>` | 7/10 | Sana | Groups synced `query_page_stats` by query; flags queries with 2+ of the site's own pages ranking; ranks by split-impression cost. |
| 5 | Quota-aware batch submit | `submit smart --file urls.txt` | 8/10 | Dev | GetUrlSubmissionQuota → SubmitUrlBatch up to remaining allowance → records to `submitted_urls` with timestamp → non-zero exit if any URL dropped. |
| 6 | Crawl-issue triage | `crawl triage <site>` | 7/10 | Priya, Marcus | Decodes synced `crawl_issues` bitflags to plain-English categories, groups by category, joins to `page_stats` so high-traffic broken pages rank first. |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| `traffic trends` | Redundant with `site health` (reports traffic delta) and `ctr-gaps`. | CTR-opportunity finder |
| `traffic query-movers` | Overlaps `ctr-gaps` and `keywords cannibalization` for the same persona. | CTR-opportunity finder |
| `keywords plan` | Thin wrapper over 3 keyword endpoints; "which of my pages rank" join folds into `keywords cannibalization`. | Keyword cannibalization detector |
| `submit history` | Bare SELECT over `submitted_urls`; ledger lives inside `submit smart` / `submit url`. | Quota-aware batch submit |
| `feeds health` | Mostly a wrapper over GetFeeds; crawl-issue join folds into `site health`. | Site health snapshot |
| `url coverage` | Batch extension of already-absorbed single-URL `url check`. | Crawl-issue triage |
| `config audit` | Speculative demand; agency cross-site value concentrates in `site triage`. | All-sites triage board |
| `keywords footprint` | Heavy overlap with `keywords cannibalization` and the pre-brief join. | Keyword cannibalization detector |
| `links trends` | `link_counts` too coarse to be a weekly signal worth its own command. | CTR-opportunity finder |
