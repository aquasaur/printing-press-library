---
name: pp-bing-webmaster-tools
description: "Every Bing Webmaster Tools API method, plus a local store, offline search, and SEO analytics no other Bing tool has... Trigger phrases: `check bing search performance for my site`, `is this url indexed by bing`, `submit urls to bing for indexing`, `bing keyword research`, `review bing crawl errors`, `use bing-webmaster-tools`, `run bing-webmaster-tools`."
author: "Cameron Bailey"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bing-webmaster-tools-pp-cli
---

# Bing Webmaster Tools — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bing-webmaster-tools-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install bing-webmaster-tools --cli-only
   ```
2. Verify: `bing-webmaster-tools-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster-tools/cmd/bing-webmaster-tools-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Bing Webmaster Tools has no official spec and no first-party CLI. This wraps the whole documented API surface — sites, sitemaps, URL and content submission, traffic and query stats, crawl control, inbound links, keyword research, and per-site config — then adds analytics commands the API itself doesn't expose: `sites health` (a scored five-endpoint summary), `sites triage` (worst-first across every site), `traffic ctr-gaps` (high-impression / low-CTR queries by estimated lost clicks), `keywords cannibalization` (queries where two of your pages compete), `crawl triage` (decoded crawl issues weighted by traffic), and `submit smart` (quota-aware batch submission with a local ledger and a CI exit code). Synced data (`sync`) and submission history land in a local SQLite store you can query with `search` and `sql`.

## When to Use This CLI

Use this CLI when an agent or operator needs to read or manage a site's presence in Bing search — pulling query and page performance, checking whether URLs are indexed and why, submitting URLs or sitemaps within quota, reviewing and triaging crawl errors, doing Bing keyword research, or auditing per-site crawl/blocking/geo config. Prefer it over a raw HTTP call when you want the decoded crawl-issue categories, the analytics commands, or a JSON shape that's already unwrapped from Bing's `{"d":...}` envelope.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Composite views over the Bing API
- **`sites health`** — One scored summary of a site's Bing health: traffic, crawl stats, top crawl issues, inbound links, and URL-submission quota — in a single command.

  _Reach for this first when an agent needs a one-shot read on how a site is doing in Bing, instead of chaining five raw endpoint calls._

  ```bash
  bing-webmaster-tools-pp-cli sites health --site-url https://navryn.com --agent
  ```
- **`sites triage`** — Ranks every verified site in your account worst-first by crawl-issue count and traffic, one row per site.

  _For agencies or anyone with many sites: one call answers 'which of my sites needs attention' without a login per site._

  ```bash
  bing-webmaster-tools-pp-cli sites triage --agent
  ```
- **`crawl triage`** — Decodes Bing's crawl-issue bitflags into plain-English categories, groups by category, and joins to page traffic so high-traffic broken pages rank first.

  _Tells an agent which crawl errors actually matter — the ones on pages people land on — instead of a flat error dump._

  ```bash
  bing-webmaster-tools-pp-cli crawl triage --site-url https://navryn.com --agent
  ```

### Search-performance analytics
- **`traffic ctr-gaps`** — Surfaces queries with high impressions but low click-through, ranked by estimated lost clicks.

  _Points an agent straight at the title/meta-description rewrites most likely to recover Bing clicks._

  ```bash
  bing-webmaster-tools-pp-cli traffic ctr-gaps --site-url https://navryn.com --min-impressions 100 --agent
  ```
- **`keywords cannibalization`** — Finds queries where two or more of your own pages both rank in Bing, ranked by the impression cost of the split.

  _Run before briefing new content so an agent doesn't create a page that competes with one that already ranks._

  ```bash
  bing-webmaster-tools-pp-cli keywords cannibalization --site-url https://navryn.com --agent
  ```

### Agent-native plumbing
- **`submit smart`** — Checks the remaining daily URL-submission quota, submits only up to that allowance, records every submission to the local store with a timestamp, and exits non-zero if any URL was dropped.

  _Safe to wire into a deploy pipeline: it never over-submits and the exit code tells CI when the day's quota ran out._

  ```bash
  bing-webmaster-tools-pp-cli submit smart --site-url https://navryn.com --file changed-urls.txt --agent
  ```

## Command Reference

**block-urls** — URLs blocked from Bing's index

- `bing-webmaster-tools-pp-cli block-urls add` — Block a URL or directory from Bing's index
- `bing-webmaster-tools-pp-cli block-urls list` — List URLs/directories blocked from Bing's index for a site
- `bing-webmaster-tools-pp-cli block-urls remove` — Unblock a previously blocked URL or directory

**crawl** — Crawl statistics, crawl issues, and crawl settings

- `bing-webmaster-tools-pp-cli crawl get-settings` — Get the crawl settings for a site
- `bing-webmaster-tools-pp-cli crawl issues` — Get crawl issues/errors for a site
- `bing-webmaster-tools-pp-cli crawl set-settings` — Update the crawl settings for a site
- `bing-webmaster-tools-pp-cli crawl stats` — Get crawl statistics (crawled pages, in-index, errors) by day for a site

**deep-links** — Deep-link blocks for app deep links

- `bing-webmaster-tools-pp-cli deep-links add` — Block deep links for a URL pattern
- `bing-webmaster-tools-pp-cli deep-links list` — List blocked deep links for a site
- `bing-webmaster-tools-pp-cli deep-links remove` — Remove a deep-link block

**feeds** — Sitemaps and RSS/Atom feeds

- `bing-webmaster-tools-pp-cli feeds list` — List sitemaps and feeds submitted for a site
- `bing-webmaster-tools-pp-cli feeds remove` — Remove a previously submitted sitemap or feed
- `bing-webmaster-tools-pp-cli feeds submit` — Submit a sitemap or feed URL for a site

**fetch** — Fetch-as-Bingbot results

- `bing-webmaster-tools-pp-cli fetch get` — Get the fetch-as-Bingbot details for a specific URL
- `bing-webmaster-tools-pp-cli fetch list` — List URLs fetched as Bingbot for a site
- `bing-webmaster-tools-pp-cli fetch url` — Queue a URL to be fetched as Bingbot

**geo** — Country/region targeting settings

- `bing-webmaster-tools-pp-cli geo add` — Add a country/region targeting setting
- `bing-webmaster-tools-pp-cli geo list` — Get country/region targeting settings for a site (may require elevated account permissions)
- `bing-webmaster-tools-pp-cli geo remove` — Remove a country/region targeting setting

**keywords** — Keyword research and historical keyword stats

- `bing-webmaster-tools-pp-cli keywords data` — Get keyword data (impressions, broad/exact match counts) for a keyword
- `bing-webmaster-tools-pp-cli keywords related` — Get keywords related to a seed keyword
- `bing-webmaster-tools-pp-cli keywords stats` — Get historical impression stats for a keyword

**links** — Inbound links and connected pages

- `bing-webmaster-tools-pp-cli links add-connected` — Add a page that links to your website
- `bing-webmaster-tools-pp-cli links connected` — List pages connected to a site (e.g. social profiles, related domains)
- `bing-webmaster-tools-pp-cli links counts` — Get inbound link counts for a site
- `bing-webmaster-tools-pp-cli links remove-connected` — Remove a connected page
- `bing-webmaster-tools-pp-cli links url` — Get inbound links for a specific site URL

**page-preview** — Page-preview (rich snippet) blocks

- `bing-webmaster-tools-pp-cli page-preview add` — Add a page-preview block to prevent rich snippets for a URL pattern
- `bing-webmaster-tools-pp-cli page-preview list` — List active page-preview blocks for a site
- `bing-webmaster-tools-pp-cli page-preview remove` — Remove a page-preview block

**query-params** — URL query-parameter normalization rules

- `bing-webmaster-tools-pp-cli query-params add` — Add a URL query-parameter normalization rule
- `bing-webmaster-tools-pp-cli query-params list` — List URL query-parameter normalization rules for a site (may require elevated account permissions)
- `bing-webmaster-tools-pp-cli query-params remove` — Remove a URL query-parameter normalization rule

**sites** — Verified sites, ownership, roles, child sites, and site moves

- `bing-webmaster-tools-pp-cli sites add` — Add a new site to your account
- `bing-webmaster-tools-pp-cli sites add-child` — Add a child site under a parent site
- `bing-webmaster-tools-pp-cli sites add-role` — Delegate site access to another user
- `bing-webmaster-tools-pp-cli sites children` — Get info about child sites under a parent site
- `bing-webmaster-tools-pp-cli sites get-roles` — List users who have access to a site
- `bing-webmaster-tools-pp-cli sites list` — List all sites registered with your Bing Webmaster Tools account
- `bing-webmaster-tools-pp-cli sites moves` — Get the site-move notifications submitted for a site
- `bing-webmaster-tools-pp-cli sites remove` — Remove a site from your account
- `bing-webmaster-tools-pp-cli sites remove-role` — Revoke a user's site access
- `bing-webmaster-tools-pp-cli sites submit-move` — Notify Bing of a site move
- `bing-webmaster-tools-pp-cli sites verify` — Verify ownership of a site

**submit** — URL and content submission for indexing, plus quotas

- `bing-webmaster-tools-pp-cli submit batch` — Submit multiple URLs for indexing in one call
- `bing-webmaster-tools-pp-cli submit content` — Submit page HTML content directly without waiting for a crawl
- `bing-webmaster-tools-pp-cli submit content-quota` — Get the content-submission quota and remaining allowance
- `bing-webmaster-tools-pp-cli submit quota` — Get the daily/monthly URL-submission quota and remaining allowance
- `bing-webmaster-tools-pp-cli submit url` — Submit a single URL for indexing

**traffic** — Search traffic, query stats, page stats, and rank/traffic time series

- `bing-webmaster-tools-pp-cli traffic pages` — Get per-page search performance for a site
- `bing-webmaster-tools-pp-cli traffic queries` — Get per-query search performance (clicks, impressions, CTR, position) for a site
- `bing-webmaster-tools-pp-cli traffic query-page-detail` — Get detailed daily stats for a specific query-page combination
- `bing-webmaster-tools-pp-cli traffic query-pages` — Get per-query, per-page search performance for a site
- `bing-webmaster-tools-pp-cli traffic stats` — Get clicks and impressions by day for a site

**url** — Per-URL index status and traffic info

- `bing-webmaster-tools-pp-cli url get` — Get index information for a specific URL
- `bing-webmaster-tools-pp-cli url traffic` — Get traffic information for a specific URL


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bing-webmaster-tools-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Site health in one call

```bash
bing-webmaster-tools-pp-cli sites health --site-url https://navryn.com --agent
```

Joins traffic, crawl stats, crawl issues, link counts, and submission quota into one scored report.

### Find Bing CTR opportunities

```bash
bing-webmaster-tools-pp-cli traffic ctr-gaps --site-url https://navryn.com --min-impressions 100 --agent
```

Lists queries getting impressions but few clicks, ranked by estimated lost clicks — your title-rewrite worklist.

### Is this URL indexed and why not

```bash
bing-webmaster-tools-pp-cli url check https://navryn.com/pricing --json
```

Joins GetUrlInfo with GetCrawlIssues and decodes the issue flags into a plain-English reason.

### Catch keyword cannibalization with a narrow payload

```bash
bing-webmaster-tools-pp-cli keywords cannibalization --site-url https://navryn.com --agent --select query,pageCount,totalImpressions,pages.page,pages.impressions
```

Finds queries where two of your pages both rank; the dotted --select keeps the agent payload to just the fields that matter.

### Post-deploy index push for CI

```bash
bing-webmaster-tools-pp-cli submit smart --site-url https://navryn.com --file changed-urls.txt --agent
```

Submits the changed URLs up to the remaining daily quota; non-zero exit if any were dropped, so the pipeline flags it.

## Auth Setup

Auth is a single API key — open the Bing Webmaster Tools dashboard, go to Settings → API access, generate a key, and set `BING_WEBMASTER_API_KEY`. The key is passed as the `apikey` query parameter on every call. Run `doctor` to confirm it is valid and the API is reachable.

Run `bing-webmaster-tools-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bing-webmaster-tools-pp-cli block-urls list --site-url https://example.com/resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
bing-webmaster-tools-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bing-webmaster-tools-pp-cli feedback --stdin < notes.txt
bing-webmaster-tools-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bing-webmaster-tools-pp-cli/feedback.jsonl`. They are never POSTed unless `BING_WEBMASTER_TOOLS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BING_WEBMASTER_TOOLS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
bing-webmaster-tools-pp-cli profile save briefing --json
bing-webmaster-tools-pp-cli --profile briefing block-urls list --site-url https://example.com/resource
bing-webmaster-tools-pp-cli profile list --json
bing-webmaster-tools-pp-cli profile show briefing
bing-webmaster-tools-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `bing-webmaster-tools-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add bing-webmaster-tools-pp-mcp -- bing-webmaster-tools-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bing-webmaster-tools-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bing-webmaster-tools-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bing-webmaster-tools-pp-cli <command> --help`.
