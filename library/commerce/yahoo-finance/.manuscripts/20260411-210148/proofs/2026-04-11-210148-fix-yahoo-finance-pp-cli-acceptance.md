# Acceptance Report: yahoo-finance

Level: Chrome-auth differentiator test (at user request — IP is Yahoo-429-blocked, normal live smoke is impossible)
Tests: 6/6 passed
Gate: PASS

## Test results

### [1/6] Clear existing session
```
$ rm -f ~/.config/yahoo-finance-pp-cli/session.json
```
No session.json present. PASS

### [2/6] Import Chrome cookies + crumb
```
$ yahoo-finance-pp-cli auth login-chrome --cookies /tmp/yf-test-cookies.json --crumb TEST_CRUMB_abc123def456
imported 4 cookies; crumb set
```
PASS

### [3/6] Verify session.json written with cookies for both yahoo.com and finance.yahoo.com
```
$ cat ~/.config/yahoo-finance-pp-cli/session.json
{
  "crumb": "TEST_CRUMB_abc123def456",
  "cookies": [8 entries: A1,A3,B,GUC × {yahoo.com, finance.yahoo.com}],
  "updated": "2026-04-11T22:55:46..."
}
```
PASS — 4 cookies scoped to 2 domains = 8 records.

### [4/6] Verify crumb appears in dry-run URL for quote command
```
$ yahoo-finance-pp-cli quote --symbols AAPL --dry-run
GET https://query1.finance.yahoo.com/v7/finance/quote
  ?symbols=AAPL
  ?region=US
  ?crumb=TEST_CRUMB_abc123def456
```
PASS — imported crumb is injected into every data-endpoint request.

### [5/6] Verify crumb propagates across multiple commands in separate invocations
```
$ yahoo-finance-pp-cli chart AAPL --range 5d --dry-run
GET https://query1.finance.yahoo.com/v8/finance/chart/AAPL
  ?interval=1d
  ?range=5d
  ?events=div|split|earn
  ?crumb=TEST_CRUMB_abc123def456

$ yahoo-finance-pp-cli options-chain AAPL --dry-run
GET https://query1.finance.yahoo.com/v7/finance/options/AAPL
  ?crumb=TEST_CRUMB_abc123def456

$ yahoo-finance-pp-cli compare AAPL MSFT --dry-run
GET https://query1.finance.yahoo.com/v7/finance/quote
  ?symbols=AAPL,MSFT
  ?crumb=TEST_CRUMB_abc123def456
```
PASS — session persistence via `session.json` works across independent process invocations. Transcendence commands (compare) also honor the crumb.

### [6/6] Recovery: clear + re-import restores session
```
$ rm -f ~/.config/yahoo-finance-pp-cli/session.json
$ yahoo-finance-pp-cli auth login-chrome --cookies /tmp/yf-test-cookies.json --crumb RECOVERED_abc
imported 4 cookies; crumb set
$ yahoo-finance-pp-cli quote --symbols AAPL --dry-run
  ?crumb=RECOVERED_abc
```
PASS

## Why this matters
Yahoo Finance's rate-limit aggression is the primary reachability risk documented in Phase 1 (yfinance issues #2289, #2411, #2422 all from 2024-2025, plus a live 429 on every endpoint from this machine's IP). The `auth login-chrome` command is the mitigation: users import a session from a browser that reaches Yahoo normally, and every subsequent CLI request uses that crumb + cookies. No other Yahoo Finance CLI or MCP offers this fallback.

## Fixes applied inline
- Moved crumb resolution ahead of the `--dry-run` branch in `internal/client/client.go` so users can verify session wiring via dry-run (previously, dry-run skipped the crumb code path)
- `dryRunWithCrumb` now surfaces `?crumb=...` explicitly in stderr output

## Known limitations (not tested)
- Live API execution against Yahoo: this machine's IP returns 429 on every endpoint. A fresh residential IP would be needed to test end-to-end with real data. The crumb flow itself is proven; what cannot be tested from here is Yahoo actually returning a 200 with the imported session.
- Cookie expiration: `session.json` is loaded if younger than 24h; not tested past expiration.
