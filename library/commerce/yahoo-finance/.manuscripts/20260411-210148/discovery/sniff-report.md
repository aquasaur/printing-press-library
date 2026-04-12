# Yahoo Finance Sniff Report

## Method
- Live IP probe: HTTP 429 on every endpoint (documented issue, mirrors yfinance #2289/#2411/#2422)
- Crumb-handshake: also 429 from this IP
- Browser sniff via Chrome: deferred — crowd-sniff + research yielded the full endpoint surface
- Crowd-sniff output: contaminated with cross-API endpoints (Polygon, FRED, Tiingo, Alpha Vantage) because npm packages like `yfin-wrapper` touch multiple financial APIs. The tool did not filter by actual domain match.

## Ground-truth endpoints (from research: yahoo-finance2 source, yfinance source, Scarvy/yahoo-finance-api-collection)

All endpoints are on `https://query1.finance.yahoo.com` or `https://query2.finance.yahoo.com`. Every request needs a valid crumb + A1/B1 cookies obtained via the bootstrap handshake below.

### Session handshake
- `GET https://fc.yahoo.com/` — establishes A1/B1 session cookies
- `GET https://query2.finance.yahoo.com/v1/test/getcrumb` — returns a crumb string, must be passed as `crumb=` query param on every data call

### Data endpoints
- `GET /v8/finance/chart/{symbol}` — OHLCV history, params: `interval`, `range`, `period1`, `period2`, `events`
- `GET /v10/finance/quoteSummary/{symbol}` — param `modules` (comma-separated): assetProfile, summaryProfile, summaryDetail, price, financialData, defaultKeyStatistics, incomeStatementHistory (Quarterly), balanceSheetHistory (Quarterly), cashflowStatementHistory (Quarterly), earnings, earningsHistory, earningsTrend, calendarEvents, secFilings, upgradeDowngradeHistory, recommendationTrend, institutionOwnership, fundOwnership, majorHoldersBreakdown, insiderHolders, insiderTransactions, netSharePurchaseActivity, indexTrend, industryTrend, sectorTrend
- `GET /v7/finance/quote` — param `symbols` (comma-separated list of tickers)
- `GET /v1/finance/search` — param `q` (query string), `quotesCount`, `newsCount`
- `GET /v7/finance/options/{symbol}` — options chain, param `date` (expiration as unix epoch)
- `GET /v1/finance/trending/{region}` — region: US, GB, FR, DE, HK, IN, AU, CA, BR, etc; param `count`
- `GET /v1/finance/screener/predefined/saved` — param `scrIds`: day_gainers, day_losers, most_actives, undervalued_growth_stocks, aggressive_small_caps, conservative_foreign_funds, high_yield_bond, portfolio_anchors, small_cap_gainers, top_mutual_funds, undervalued_large_caps, growth_technology_stocks
- `POST /v1/finance/screener` — custom screener with body containing operator/operands filter DSL
- `GET /v1/finance/recommendationsbysymbol/{symbol}` — similar symbols
- `GET /ws/insights/v3/finance/insights` — param `symbol`, returns company insights / valuation
- `GET /ws/fundamentals/v1/finance/timeseries/{symbol}` — params `type` (comma-separated fundamentals keys), `period1`, `period2`
- `GET /v6/finance/autocomplete` — legacy autocomplete, param `query`, `lang`, `region`
- `GET /v2/finance/trending/{region}` — alternative trending endpoint

### Response envelope
Every response wraps payload in `{<moduleName>: {result: [...], error: null}}`. For example:
```
{ "chart": { "result": [{...}], "error": null } }
{ "quoteSummary": { "result": [{...}], "error": null } }
{ "quoteResponse": { "result": [...], "error": null } }
```
This is the proxy-envelope pattern and must be handled consistently by the generated client.

### Rate-limit behavior
- HTTP 429 with body "Too Many Requests" (19 bytes literal)
- Exponential backoff required: 1s, 2s, 4s, 8s, 16s
- Crumb expires; re-bootstrap if stale crumb returns 401 or 403

## Recommendation
Hand-authored internal YAML spec using the endpoints above. Crowd-sniff output kept for audit trail at `research/yahoo-finance-crowd-spec.yaml` but not used as primary spec (too noisy).
