# Yahoo Finance CLI

**Every Yahoo Finance feature, plus the things every existing tool is missing: portfolio tracking, watchlist-driven digests, options moneyness filters, and a Chrome-session fallback for when Yahoo blocks your IP.**

Quotes, charts, fundamentals, options chains, screeners, trending symbols, insights, analyst recommendations, and a SQLite-backed local state layer that competitors don't have. No API key required. Works offline once synced.

## Why this CLI?

Every other Yahoo Finance tool is one of three things:

| Tool type | Example | What's missing |
|-----------|---------|----------------|
| Library | `yfinance` (14k★), `yahoo-finance2` (2.8k★) | Not a CLI. No agent-native flags. No local state. No composable pipelines. |
| Tiny CLI | `yf`, `yahoo-finance-cli`, `stock-quote` | Quote-only or CSV-only. No fundamentals depth, no options filtering, no portfolios. |
| MCP server | `Alex2Yang97/yahoo-finance-mcp` (262★) | Great for Claude Desktop — not for shell pipelines, cron jobs, or scripts. |

**This CLI is the first to combine:**

1. **Every feature from every wrapper** — quote, chart, quoteSummary (25 submodules), options, screener, trending, fundamentals, insights, recommendations, search, autocomplete. Matches yfinance's full surface area.
2. **Portfolio and watchlist state that persists** — SQLite-backed lots, cost basis, and returns. Query with raw SQL. Nothing else does this.
3. **Transcendence commands that only make sense with local state** — `digest`, `portfolio perf`, `options-chain --moneyness`, `compare`, `sparkline`, cross-entity `sql`.
4. **Agent-native output on every command** — `--json`, `--csv`, `--select`, `--compact`, `--dry-run`, typed exit codes. Pipe to `jq`, script around it, run under cron.
5. **Resilient to Yahoo's rate-limit aggression** — the CLI handles the crumb+cookie handshake automatically, caches the session to disk, backs off adaptively on 429s, and — uniquely — lets you import a real Chrome session (`auth login-chrome`) when Yahoo blocks your IP outright.

Powered by the reverse-engineered endpoints at `query1.finance.yahoo.com` and `query2.finance.yahoo.com` that yfinance, yahoo-finance2, and yahooquery have proven over the past decade.

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Authentication

Yahoo Finance has no official API and no API key. The CLI bootstraps a session automatically by visiting `fc.yahoo.com` and fetching a crumb from `/v1/test/getcrumb` — the same pattern every successful wrapper uses.

If your IP is rate-limited (common on cloud providers and some residential ISPs — Yahoo returns HTTP 429 on every request), you can import a real browser session instead:

```bash
# 1. Open finance.yahoo.com in Chrome, accept cookies, stay signed out.
# 2. Export cookies for *.yahoo.com as JSON (use a cookie-export extension).
# 3. Get a crumb from the browser DevTools console on finance.yahoo.com:
#    fetch('/v1/test/getcrumb').then(r => r.text()).then(console.log)
yahoo-finance-pp-cli auth login-chrome --cookies ~/yahoo-cookies.json --crumb abc123
```

## Quick Start

```bash
# 1. Verify setup (checks the crumb handshake works from your IP)
yahoo-finance-pp-cli doctor

# 2. Your first quote
yahoo-finance-pp-cli quote AAPL MSFT NVDA

# 3. Build a watchlist and check it daily
yahoo-finance-pp-cli watchlist create tech
yahoo-finance-pp-cli watchlist add tech AAPL MSFT NVDA GOOG
yahoo-finance-pp-cli digest --watchlist tech

# 4. Track a portfolio
yahoo-finance-pp-cli portfolio add AAPL 50 185.50 --purchased 2024-06-15
yahoo-finance-pp-cli portfolio perf

# 5. Options chain filtered to what actually matters
yahoo-finance-pp-cli options-chain AAPL --moneyness otm --max-dte 45 --type calls
```

## Unique Features

These capabilities aren't available in any other Yahoo Finance CLI, library, or MCP.

### Local state that compounds

- **`portfolio add SYMBOL SHARES COST --purchased YYYY-MM-DD`** — Record a purchase lot. Multiple lots per symbol are fine.
- **`portfolio perf`** — Current market value, cost basis, unrealized P&L per position, and portfolio total. Uses live quotes joined with your local lots.
- **`portfolio gains`** — Per-lot unrealized gain/loss sorted by magnitude. Useful for tax-lot selection.
- **`watchlist create|add|show|list`** — Named collections of tickers backed by SQLite. Feed them into multi-symbol commands.
- **`sql "..."`** — Raw SQLite against the local database. `SELECT symbol, SUM(shares*cost_basis) FROM portfolio_lots GROUP BY symbol` style queries work out of the box.

### Commands that only make sense with local state

- **`digest --watchlist tech`** — One command returns biggest gainers and losers across your watchlist. Morning briefing in a single line.
- **`compare AAPL MSFT NVDA GOOG`** — Side-by-side quote + 52w range + market cap across any number of symbols, parallel-fetched.
- **`sparkline AAPL --range 1mo`** — Unicode sparkline (`▁▂▃▄▅▆▇█`) of recent price action. Zero-config terminal chart.
- **`options-chain AAPL --moneyness otm --max-dte 45 --type calls`** — Options chain filtered to out-of-the-money calls expiring within 45 days. Yahoo's raw endpoint returns everything; this command does the filtering you actually want.
- **`fx USD EUR --amount 100`** — Currency conversion using Yahoo Finance's FX pairs. 100 USD to EUR in one line.

### Agent-native plumbing on every command

- `--json` — pipe to `jq`, feed LLMs, build scripts
- `--csv` — spreadsheet import
- `--compact` — minimum-token output for agent contexts
- `--select id,name,price` — cherry-pick fields
- `--dry-run` — preview the HTTP request (including the crumb) without sending
- `--data-source auto|live|local` — force live or offline mode
- Typed exit codes: `0` success, `3` not found, `5` API error, `7` rate limited

### Reachability mitigation

- **`auth login-chrome`** — when Yahoo 429s your IP (common on cloud IPs and some residential ISPs), import a live Chrome session and the CLI uses its crumb + cookies instead. No other Yahoo Finance tool has this.
- **Adaptive rate limiter** — starts conservative, ramps up on success, halves on 429, persists the ceiling per-session.
- **Automatic crumb bootstrap** — the fc.yahoo.com → getcrumb handshake every working wrapper does, built-in and cached to disk for 24 hours.

## Usage

<!-- HELP_OUTPUT -->

## Commands

### autocomplete

Legacy autocomplete (faster than search)

- **`yahoo-finance-pp-cli autocomplete get`** - Autocomplete symbols and company names

### chart

Historical OHLCV price data

- **`yahoo-finance-pp-cli chart get`** - Historical price chart data for a symbol

### fundamentals

Time series of fundamentals (quarterly/annual)

- **`yahoo-finance-pp-cli fundamentals timeseries`** - Fundamentals time series (EPS, revenue, margin, cash flow, etc.)

### insights

Company insights, valuation, and technical events

- **`yahoo-finance-pp-cli insights get`** - Insights for a symbol: technical events, valuation, research reports

### options

Options chains for equities and ETFs

- **`yahoo-finance-pp-cli options get`** - Options chain for a symbol (calls and puts)

### quote

Real-time quotes and quote summaries

- **`yahoo-finance-pp-cli quote list`** - Get current quotes for one or more symbols
- **`yahoo-finance-pp-cli quote summary`** - Deep quote summary including price, fundamentals, ownership, and filings

### recommendations

Symbols related by analyst recommendation

- **`yahoo-finance-pp-cli recommendations bysymbol`** - Symbols that share recommendations with the given symbol

### screener

Predefined and custom stock screeners

- **`yahoo-finance-pp-cli screener predefined`** - Run a predefined screener by ID

### search

Symbol search and autocomplete

- **`yahoo-finance-pp-cli search query`** - Search for symbols, news, and funds matching a query

### trending

Trending symbols by region

- **`yahoo-finance-pp-cli trending list`** - Top trending symbols in a region right now


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
yahoo-finance-pp-cli autocomplete list

# JSON for scripting and agents
yahoo-finance-pp-cli autocomplete list --json

# Filter to specific fields
yahoo-finance-pp-cli autocomplete list --json --select id,name,status

# Dry run — show the request without sending
yahoo-finance-pp-cli autocomplete list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
yahoo-finance-pp-cli autocomplete list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | yahoo-finance-pp-cli <resource> create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add yahoo-finance yahoo-finance-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "yahoo-finance": {
      "command": "yahoo-finance-pp-mcp"
    }
  }
}
```

## Cookbook

Common workflows and recipes:

```bash
# List resources as JSON for scripting
yahoo-finance-pp-cli autocomplete list --json

# Filter to specific fields
yahoo-finance-pp-cli autocomplete list --json --select id,name,status

# Dry run to preview the request
yahoo-finance-pp-cli autocomplete list --dry-run

# Sync data locally for offline search
yahoo-finance-pp-cli sync

# Search synced data
yahoo-finance-pp-cli search "query"

# Export for backup
yahoo-finance-pp-cli export --format jsonl > backup.jsonl
```

## Health Check

```bash
yahoo-finance-pp-cli doctor
```

<!-- DOCTOR_OUTPUT -->

## Configuration

Config file: `~/.config/yahoo-finance-pp-cli/config.toml`

Environment variables:

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `yahoo-finance-pp-cli doctor` to check credentials

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- If persistent, wait a few minutes and try again

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**yfinance**](https://github.com/ranaroussi/yfinance) — Python (14000 stars)
- [**yahoo-finance2**](https://github.com/gadicc/yahoo-finance2) — JavaScript (2800 stars)
- [**yahooquery**](https://github.com/dpguthrie/yahooquery) — Python (1000 stars)
- [**Alex2Yang97/yahoo-finance-mcp**](https://github.com/Alex2Yang97/yahoo-finance-mcp) — Python (262 stars)
- [**kanishka-namdeo/yfnhanced-mcp**](https://github.com/kanishka-namdeo/yfnhanced-mcp) — Python
- [**BillGatesCat/yf**](https://github.com/BillGatesCat/yf) — Go
- [**tabrindle/yahoo-finance-cli**](https://github.com/tabrindle/yahoo-finance-cli) — JavaScript
- [**Scarvy/yahoo-finance-api-collection**](https://github.com/Scarvy/yahoo-finance-api-collection) — Bruno

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
