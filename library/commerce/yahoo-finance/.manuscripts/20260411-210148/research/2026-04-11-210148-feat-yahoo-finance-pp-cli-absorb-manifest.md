# Yahoo Finance GOAT — Absorb Manifest

## Tools Cataloged

| Tool | Type | Stars | Source |
|------|------|-------|--------|
| yfinance (ranaroussi) | Python library | ~14k | https://github.com/ranaroussi/yfinance |
| yahoo-finance2 (gadicc) | Node.js library | ~2.8k | https://github.com/gadicc/yahoo-finance2 |
| Alex2Yang97/yahoo-finance-mcp | MCP server | 262 | https://github.com/Alex2Yang97/yahoo-finance-mcp |
| AgentX-ai/yahoo-finance-server | MCP server | — | https://github.com/AgentX-ai/yahoo-finance-server |
| kanishka-namdeo/yfnhanced-mcp | MCP server (caching+circuit breaker) | — | https://github.com/kanishka-namdeo/yfnhanced-mcp |
| BillGatesCat/yf | Go CLI | — | https://github.com/BillGatesCat/yf |
| tabrindle/yahoo-finance-cli | Node CLI (CSV-API) | — | https://github.com/tabrindle/yahoo-finance-cli |
| scottjbarr/yahoofinance | Go library+CLI | — | https://github.com/scottjbarr/yahoofinance |
| Scarvy/yahoo-finance-api-collection | Endpoint collection (Bruno) | — | https://github.com/Scarvy/yahoo-finance-api-collection |
| ghulette/stock-quote | CLI | — | https://github.com/ghulette/stock-quote |
| boyank/yoc | Options scraper CLI | — | https://github.com/boyank/yoc |
| dpguthrie/yahooquery | Python library | ~1k | https://yahooquery.dpguthrie.com |

## Absorbed (match or beat everything that exists)

### Real-time and near-real-time data
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 1 | Current quote (single or many symbols) | yahoo-finance2 `quote()`, yfinance `fast_info` | `quote AAPL MSFT NVDA` | `--json`, `--csv`, `--compact`, `--select`; supports 100+ symbols in one call with chunking |
| 2 | Watchlist quotes | MCP `get_stock_info` | `watchlist show tech` | Query local SQLite watchlist, dedupe symbols, format as table/json |
| 3 | Pre/post-market price | yfinance | `quote AAPL --extended-hours` | Single flag toggles the `includePrePost` endpoint param |
| 4 | Currency-normalized quote | scottjbarr `yahoofinance` | `quote SONY --in-currency USD` | Auto-fetch FX pair and convert |

### Historical data (chart)
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 5 | Historical OHLCV | yfinance `history()`, yahoo-finance2 `chart()` | `history AAPL --range 1y` | Auto-caches to SQLite, dedup on upsert; `--interval` supports 1m-3mo |
| 6 | Dividend history | yfinance `dividends` | `dividends AAPL` | SQL-composable: `SELECT sum(amount) FROM dividends WHERE symbol='AAPL' AND ex_date > date('now','-1 year')` |
| 7 | Split history | yfinance `splits` | `splits AAPL` | Same offline query path |
| 8 | Corporate actions (combined) | yfinance `actions` | `actions AAPL` | Unified view of divs + splits + capital gains |
| 9 | Historical download (bulk) | yfinance `download()` | `sync history --symbols file:watchlist.txt` | Concurrent fetches with rate-limit-aware pool |
| 10 | Capital gains | yfinance `capital_gains` | `history AAPL --events capitalgains` | Single endpoint param |

### Fundamentals / financials
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 11 | Income statement (annual) | yfinance `income_stmt`, yahoo-finance2 `quoteSummary.incomeStatementHistory` | `financials AAPL --statement income` | Human-readable table; SQL-queryable; `--period annual|quarterly|ttm` |
| 12 | Balance sheet | yfinance `balance_sheet` | `financials AAPL --statement balance-sheet` | Same |
| 13 | Cash flow statement | yfinance `cashflow` | `financials AAPL --statement cashflow` | Same |
| 14 | Quarterly income | yfinance `quarterly_income_stmt` | `financials AAPL --statement income --period quarterly` | Same |
| 15 | TTM income | yfinance `ttm_income_stmt` | `financials AAPL --statement income --period ttm` | Same |
| 16 | Key statistics | yahoo-finance2 `defaultKeyStatistics` | `stats AAPL` | P/E, EPS, market cap, forward P/E, PEG, 52w range, institutional % etc. |
| 17 | Asset/company profile | yahoo-finance2 `assetProfile` | `profile AAPL` | Description, sector, industry, officers, address |
| 18 | Earnings history and estimates | yfinance `earnings_estimate`, yahoo-finance2 `earnings` | `earnings AAPL --history` | Shows actual vs estimate per quarter |
| 19 | Earnings calendar | yahoo-finance2 `calendarEvents` | `calendar AAPL` | Next earnings date, ex-div dates |
| 20 | SEC filings | yahoo-finance2 `secFilings` | `filings AAPL` | Recent 10-K, 10-Q, 8-K links |
| 21 | Time-series fundamentals | yahoo-finance2 `fundamentalsTimeSeries` | `fundamentals AAPL --keys revenue,eps --period annual` | Direct hits to `/ws/fundamentals/v1/finance/timeseries/` |

### Ownership and insider data
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 22 | Institutional holders | yfinance `institutional_holders` | `holders AAPL --type institutional` | Table with shares, value, pct, date |
| 23 | Mutual fund holders | yfinance `mutualfund_holders` | `holders AAPL --type fund` | Same |
| 24 | Major holders breakdown | yahoo-finance2 `majorHoldersBreakdown` | `holders AAPL --type major` | Insider % vs institution % |
| 25 | Insider transactions | yfinance `insider_purchases`, yahoo-finance2 `insiderTransactions` | `insiders AAPL` | Recent buy/sell by officers |
| 26 | Insider holders | yahoo-finance2 `insiderHolders` | `insiders AAPL --type holders` | Current insider holdings |
| 27 | Net share purchase activity | yahoo-finance2 `netSharePurchaseActivity` | `insiders AAPL --activity` | Net buys vs sells trend |

### Analyst data
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 28 | Analyst recommendations | yfinance `recommendations` | `analysts AAPL` | Recent upgrades/downgrades, firm, action |
| 29 | Recommendation summary | yfinance `recommendations_summary` | `analysts AAPL --summary` | Buy/hold/sell counts |
| 30 | Price targets | yfinance `analyst_price_targets` | `analysts AAPL --targets` | Low/mean/high, consensus |
| 31 | Upgrade/downgrade history | yahoo-finance2 `upgradeDowngradeHistory` | `analysts AAPL --history` | Full history |
| 32 | Recommendations by symbol (peers) | yahoo-finance2 `recommendationsBySymbol` | `peers AAPL` | Symbols analysts recommend alongside this one |

### Options
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 33 | Expirations list | yahoo-finance2 `options` | `options AAPL --expirations` | Simple enumeration |
| 34 | Options chain | yfinance `option_chain` | `options AAPL --expiry 2026-04-18` | Calls + puts with IV, OI, volume |
| 35 | Calls only | yfinance `option_chain().calls` | `options AAPL --calls` | Filter |
| 36 | Puts only | yfinance `option_chain().puts` | `options AAPL --puts` | Filter |
| 37 | Filter by strike range | boyank/yoc | `options AAPL --min-strike 150 --max-strike 200` | |
| 38 | ATM contracts | (none direct) | `options AAPL --moneyness atm` | Computed relative to spot |

### News
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 39 | Per-symbol news | yfinance `news`, yahoo-finance2 via search | `news AAPL` | Title, publisher, date, URL |
| 40 | Multi-symbol news digest | (none) | `news --watchlist tech` | Aggregates across watchlist, dedupes |
| 41 | Offline news search | — | `news search "earnings beat"` | FTS5 on locally synced news |

### Discovery
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 42 | Symbol search | yahoo-finance2 `search()`, yfinance | `search apple` | Fuzzy match tickers + companies |
| 43 | Autocomplete | yahoo-finance2 `autoc()` | `search --autocomplete app` | Faster prefix match |
| 44 | Trending symbols | yahoo-finance2 `trendingSymbols` | `trending [region]` | Top symbols by region |
| 45 | Day gainers | yahoo-finance2 `dailyGainers` | `screen day-gainers` | Predefined screener wrapper |
| 46 | Day losers | yahoo-finance2 `dailyLosers` | `screen day-losers` | |
| 47 | Most actives | Predefined screener | `screen most-actives` | |
| 48 | Undervalued large caps | Predefined screener | `screen undervalued-large-caps` | |
| 49 | Growth tech stocks | Predefined screener | `screen growth-tech` | |
| 50 | All predefined screeners (12+) | — | `screen list` | Enumerate available |
| 51 | Insights / research | yahoo-finance2 `insights()` | `insights AAPL` | Technical events, valuation |

### Misc
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 52 | ISIN lookup | yfinance `isin` | `profile AAPL --isin` | Derived or queried |
| 53 | Currency conversion | ivanrad/yahoofx | `fx USD EUR --amount 100` | Uses `EURUSD=X` chart |
| 54 | FX rates snapshot | — | `fx rates` | Reads a curated list of pairs |

---

## Transcendence (only possible with our approach)

### User-first feature discovery — personas and rituals

**Persona 1: Retail investor tracking a portfolio.** Morning ritual: check overnight news on their holdings, YTD performance, dividend income this year, upcoming earnings. Frustration: every CLI shows one ticker at a time; no tool aggregates "my portfolio" across requests.

**Persona 2: Options trader looking for weekly income.** Ritual: scan for stocks with high IV near earnings, check the weekly chain for ATM credit spreads. Frustration: pulling a chain then eyeballing strike/IV manually.

**Persona 3: Dividend growth investor.** Ritual: screen for stocks with rising dividends, check ex-div calendar, track YTD dividend income. Frustration: no CLI computes dividend income over a holding period.

**Persona 4: Long-term value investor / fundamentals-focused.** Ritual: read financials, compare P/E vs peers, check insider buying. Frustration: no tool joins fundamentals with insider activity in one query.

### Transcendence commands

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| T1 | Portfolio return tracker | `portfolio perf --period ytd` | Joins local `portfolio_lots` table (cost basis, shares, purchase date) with live quotes and historical dividends. No public tool maintains cost-basis state between calls. | 9/10 |
| T2 | Dividend income report | `portfolio dividends --year 2026` | Joins local lots with locally-synced dividends history; computes income per holding, totals, yield on cost. Persona 3's killer feature. | 9/10 |
| T3 | Cost-basis gain/loss | `portfolio gains --unrealized` | Computes each lot's P&L at current quote; sorts by magnitude; exports to CSV for tax prep. | 8/10 |
| T4 | Earnings this week (my holdings) | `earnings-calendar --watchlist main` | Cross-joins the local watchlist with `calendarEvents` data. Only works with SQLite state. | 8/10 |
| T5 | Morning digest | `digest --watchlist main` | Aggregates overnight news + biggest movers + earnings today + dividend ex-dates across the user's symbols. No API call returns this. Persona 1's killer morning command. | 9/10 |
| T6 | Watchlist CRUD | `watchlist create`, `add`, `remove`, `list`, `show` | Local SQLite concept — no API equivalent. | 7/10 |
| T7 | Terminal sparkline chart | `history AAPL --sparkline` | ASCII sparkline from cached OHLCV — no network needed after first sync. | 6/10 |
| T8 | Options moneyness filter | `options AAPL --moneyness otm --max-dte 45` | Computes spot vs strike per contract; no API filter does this. Persona 2's killer command. | 8/10 |
| T9 | Local screener over synced universe | `screen-local --pe-max 20 --pb-max 3 --div-yield-min 2` | Runs SQL filters against the `quotes` + `key_statistics` tables. Yahoo's remote screener is limited to 12 predefined IDs; this one is arbitrary. | 8/10 |
| T10 | Peer comparison | `compare AAPL MSFT GOOG --metric pe,ebitda,margin` | Parallel fetch + side-by-side table. No tool does this for Yahoo Finance today. | 7/10 |
| T11 | Offline-first SQL query | `sql "SELECT symbol, close FROM history WHERE date=date('now','-1 day') ORDER BY close DESC LIMIT 10"` | Direct SQLite query access — unique to our stack. | 7/10 |
| T12 | Symbol FTS search offline | `search --offline tesla` | Against synced symbol index — fast, works on a plane. | 6/10 |
| T13 | Insider-buying screener | `insiders --recent 30d --net-buying` | Joins recent insider transactions across local holdings + filters to net-buying companies. Persona 4's killer signal. | 8/10 |
| T14 | Dividend stock screener | `screen-div --min-yield 3 --min-growth-years 5` | SQL over cached dividend history; identifies "dividend growers" via consecutive-year streak logic. | 8/10 |
| T15 | Chrome cookie fallback | `auth login --chrome` | When the crumb dance fails from the user's IP, import cookies from an active Chrome profile. No other Yahoo Finance tool does this. | 8/10 |
| T16 | Rate-limit-aware sync | `sync history --symbols file:tickers.txt --concurrency 4 --backoff` | Respects 429s with exponential backoff and crumb re-bootstrap. Keeps going where yfinance gives up. | 8/10 |

**Total features: 54 absorbed + 16 transcendence = 70**

That beats every single existing CLI (largest is yf at ~10 commands) and every single MCP (largest is Alex2Yang97 at 9 tools). Only yfinance as a library has more entry points, but it's a library — no agent-native CLI interface, no local state, no offline, no watchlists, no portfolio.

---

## Build Priorities

- **P0 (foundation):** spec-driven resources, data layer (quotes, history, dividends, splits, options_chains, financials, recommendations, holders, news, watchlists, portfolio_lots, symbols FTS), sync infra, crumb+cookie client, `auth login --chrome`, rate-limit handler
- **P1 (absorb):** all 54 absorbed commands — `quote`, `quote-summary`/`summary`, `history`, `dividends`, `splits`, `actions`, `options`, `search`, `autocomplete`, `trending`, `screen` (with all 12 predefined IDs), `insights`, `fundamentals`, `financials`, `stats`, `profile`, `earnings`, `calendar`, `filings`, `holders` (all 4 types), `insiders`, `analysts` (recommendations + summary + targets + history), `peers`, `news`, `fx`
- **P2 (transcend):** T1-T16 — all 16 commands above
- **P3 (polish):** enriched flag help text, sparkline fidelity, README cookbook, csv output for all tables, `--select` for every command

## Auth & Session Handling Note
- Yahoo Finance has no official auth but REQUIRES a crumb+cookie handshake on every request
- Client must: GET `https://fc.yahoo.com/` → persist A1/B1 cookies → GET `/v1/test/getcrumb` → include `crumb=...` on every data call
- Hand-implemented in Phase 3 since the generator's standard auth types don't cover session handshakes
- `auth login --chrome` as a fallback when user's residential/cloud IP is 429-blocked
