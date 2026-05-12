# Bing Webmaster Tools CLI

**Every Bing Webmaster Tools API method, plus a local store, offline search, and SEO analytics no other Bing tool has — built agent-native.**

Bing Webmaster Tools has no official spec and no first-party CLI. This wraps the whole documented API surface — sites, sitemaps, URL and content submission, traffic and query stats, crawl control, inbound links, keyword research, and per-site config — then adds analytics commands the API itself doesn't expose: `sites health` (a scored five-endpoint summary), `sites triage` (worst-first across every site), `traffic ctr-gaps` (high-impression / low-CTR queries by estimated lost clicks), `keywords cannibalization` (queries where two of your pages compete), `crawl triage` (decoded crawl issues weighted by traffic), and `submit smart` (quota-aware batch submission with a local ledger and a CI exit code). Synced data (`sync`) and submission history land in a local SQLite store you can query with `search` and `sql`.

## Install

The recommended path installs both the `bing-webmaster-tools-pp-cli` binary and the `pp-bing-webmaster-tools` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install bing-webmaster-tools
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install bing-webmaster-tools --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bing-webmaster-tools-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bing-webmaster-tools --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bing-webmaster-tools --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-bing-webmaster-tools skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-bing-webmaster-tools. The skill defines how its required CLI can be installed.
```

## Authentication

Auth is a single API key — open the Bing Webmaster Tools dashboard, go to Settings → API access, generate a key, and set `BING_WEBMASTER_API_KEY`. The key is passed as the `apikey` query parameter on every call. Run `doctor` to confirm it is valid and the API is reachable.

## Quick Start

```bash
# Confirm BING_WEBMASTER_API_KEY is set and ssl.bing.com is reachable.
bing-webmaster-tools-pp-cli doctor


# List the sites in your account — you need the site URL for everything below.
bing-webmaster-tools-pp-cli sites list


# One scored summary: traffic, crawl, top issues, links, quota.
bing-webmaster-tools-pp-cli sites health --site-url https://navryn.com


# Find the high-impression / low-CTR queries worth a title rewrite.
bing-webmaster-tools-pp-cli traffic ctr-gaps --site-url https://navryn.com --min-impressions 100


# Is this URL indexed by Bing — and if not, what's the crawl reason?
bing-webmaster-tools-pp-cli url check https://navryn.com/pricing


# Submit changed URLs for indexing without blowing the daily quota.
bing-webmaster-tools-pp-cli submit smart --site-url https://navryn.com --file changed-urls.txt

```

## Unique Features

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

## Usage

Run `bing-webmaster-tools-pp-cli --help` for the full command reference and flag list.

## Commands

### block-urls

URLs blocked from Bing's index

- **`bing-webmaster-tools-pp-cli block-urls add`** - Block a URL or directory from Bing's index
- **`bing-webmaster-tools-pp-cli block-urls list`** - List URLs/directories blocked from Bing's index for a site
- **`bing-webmaster-tools-pp-cli block-urls remove`** - Unblock a previously blocked URL or directory

### crawl

Crawl statistics, crawl issues, and crawl settings

- **`bing-webmaster-tools-pp-cli crawl get-settings`** - Get the crawl settings for a site
- **`bing-webmaster-tools-pp-cli crawl issues`** - Get crawl issues/errors for a site
- **`bing-webmaster-tools-pp-cli crawl set-settings`** - Update the crawl settings for a site
- **`bing-webmaster-tools-pp-cli crawl stats`** - Get crawl statistics (crawled pages, in-index, errors) by day for a site

### deep-links

Deep-link blocks for app deep links

- **`bing-webmaster-tools-pp-cli deep-links add`** - Block deep links for a URL pattern
- **`bing-webmaster-tools-pp-cli deep-links list`** - List blocked deep links for a site
- **`bing-webmaster-tools-pp-cli deep-links remove`** - Remove a deep-link block

### feeds

Sitemaps and RSS/Atom feeds

- **`bing-webmaster-tools-pp-cli feeds list`** - List sitemaps and feeds submitted for a site
- **`bing-webmaster-tools-pp-cli feeds remove`** - Remove a previously submitted sitemap or feed
- **`bing-webmaster-tools-pp-cli feeds submit`** - Submit a sitemap or feed URL for a site

### fetch

Fetch-as-Bingbot results

- **`bing-webmaster-tools-pp-cli fetch get`** - Get the fetch-as-Bingbot details for a specific URL
- **`bing-webmaster-tools-pp-cli fetch list`** - List URLs fetched as Bingbot for a site
- **`bing-webmaster-tools-pp-cli fetch url`** - Queue a URL to be fetched as Bingbot

### geo

Country/region targeting settings

- **`bing-webmaster-tools-pp-cli geo add`** - Add a country/region targeting setting
- **`bing-webmaster-tools-pp-cli geo list`** - Get country/region targeting settings for a site (may require elevated account permissions)
- **`bing-webmaster-tools-pp-cli geo remove`** - Remove a country/region targeting setting

### keywords

Keyword research and historical keyword stats

- **`bing-webmaster-tools-pp-cli keywords data`** - Get keyword data (impressions, broad/exact match counts) for a keyword
- **`bing-webmaster-tools-pp-cli keywords related`** - Get keywords related to a seed keyword
- **`bing-webmaster-tools-pp-cli keywords stats`** - Get historical impression stats for a keyword

### links

Inbound links and connected pages

- **`bing-webmaster-tools-pp-cli links add-connected`** - Add a page that links to your website
- **`bing-webmaster-tools-pp-cli links connected`** - List pages connected to a site (e.g. social profiles, related domains)
- **`bing-webmaster-tools-pp-cli links counts`** - Get inbound link counts for a site
- **`bing-webmaster-tools-pp-cli links remove-connected`** - Remove a connected page
- **`bing-webmaster-tools-pp-cli links url`** - Get inbound links for a specific site URL

### page-preview

Page-preview (rich snippet) blocks

- **`bing-webmaster-tools-pp-cli page-preview add`** - Add a page-preview block to prevent rich snippets for a URL pattern
- **`bing-webmaster-tools-pp-cli page-preview list`** - List active page-preview blocks for a site
- **`bing-webmaster-tools-pp-cli page-preview remove`** - Remove a page-preview block

### query-params

URL query-parameter normalization rules

- **`bing-webmaster-tools-pp-cli query-params add`** - Add a URL query-parameter normalization rule
- **`bing-webmaster-tools-pp-cli query-params list`** - List URL query-parameter normalization rules for a site (may require elevated account permissions)
- **`bing-webmaster-tools-pp-cli query-params remove`** - Remove a URL query-parameter normalization rule

### sites

Verified sites, ownership, roles, child sites, and site moves

- **`bing-webmaster-tools-pp-cli sites add`** - Add a new site to your account
- **`bing-webmaster-tools-pp-cli sites add-child`** - Add a child site under a parent site
- **`bing-webmaster-tools-pp-cli sites add-role`** - Delegate site access to another user
- **`bing-webmaster-tools-pp-cli sites children`** - Get info about child sites under a parent site
- **`bing-webmaster-tools-pp-cli sites get-roles`** - List users who have access to a site
- **`bing-webmaster-tools-pp-cli sites list`** - List all sites registered with your Bing Webmaster Tools account
- **`bing-webmaster-tools-pp-cli sites moves`** - Get the site-move notifications submitted for a site
- **`bing-webmaster-tools-pp-cli sites remove`** - Remove a site from your account
- **`bing-webmaster-tools-pp-cli sites remove-role`** - Revoke a user's site access
- **`bing-webmaster-tools-pp-cli sites submit-move`** - Notify Bing of a site move
- **`bing-webmaster-tools-pp-cli sites verify`** - Verify ownership of a site

### submit

URL and content submission for indexing, plus quotas

- **`bing-webmaster-tools-pp-cli submit batch`** - Submit multiple URLs for indexing in one call
- **`bing-webmaster-tools-pp-cli submit content`** - Submit page HTML content directly without waiting for a crawl
- **`bing-webmaster-tools-pp-cli submit content-quota`** - Get the content-submission quota and remaining allowance
- **`bing-webmaster-tools-pp-cli submit quota`** - Get the daily/monthly URL-submission quota and remaining allowance
- **`bing-webmaster-tools-pp-cli submit url`** - Submit a single URL for indexing

### traffic

Search traffic, query stats, page stats, and rank/traffic time series

- **`bing-webmaster-tools-pp-cli traffic pages`** - Get per-page search performance for a site
- **`bing-webmaster-tools-pp-cli traffic queries`** - Get per-query search performance (clicks, impressions, CTR, position) for a site
- **`bing-webmaster-tools-pp-cli traffic query-page-detail`** - Get detailed daily stats for a specific query-page combination
- **`bing-webmaster-tools-pp-cli traffic query-pages`** - Get per-query, per-page search performance for a site
- **`bing-webmaster-tools-pp-cli traffic stats`** - Get clicks and impressions by day for a site

### url

Per-URL index status and traffic info

- **`bing-webmaster-tools-pp-cli url get`** - Get index information for a specific URL
- **`bing-webmaster-tools-pp-cli url traffic`** - Get traffic information for a specific URL


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bing-webmaster-tools-pp-cli block-urls list --site-url https://example.com/resource

# JSON for scripting and agents
bing-webmaster-tools-pp-cli block-urls list --site-url https://example.com/resource --json

# Filter to specific fields
bing-webmaster-tools-pp-cli block-urls list --site-url https://example.com/resource --json --select id,name,status

# Dry run — show the request without sending
bing-webmaster-tools-pp-cli block-urls list --site-url https://example.com/resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bing-webmaster-tools-pp-cli block-urls list --site-url https://example.com/resource --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-bing-webmaster-tools -g
```

Then invoke `/pp-bing-webmaster-tools <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add bing-webmaster-tools bing-webmaster-tools-pp-mcp -e BING_WEBMASTER_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bing-webmaster-tools-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BING_WEBMASTER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bing-webmaster-tools": {
      "command": "bing-webmaster-tools-pp-mcp",
      "env": {
        "BING_WEBMASTER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
bing-webmaster-tools-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/bing-webmaster-tools-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BING_WEBMASTER_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bing-webmaster-tools-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BING_WEBMASTER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`ERROR!!! InvalidApiKey` or HTTP 400 on every call** — Your apikey is wrong or missing — copy it again from the dashboard's Settings → API access page and re-export BING_WEBMASTER_API_KEY.
- **`submit url` fails with a quota error** — Run `submit quota --site-url <site>` to see the remaining daily allowance; new accounts often get ~10/day. Use `submit smart` to stay under it, or switch real-time submission to IndexNow.
- **`search` or `sql` returns nothing** — Run `sync` first — those commands read the local store, which starts empty. Note that stat resources require a site URL, so `sync` only populates the account-wide tables (sites) plus anything `submit smart` has recorded.
- **Dates in `--json` look like `/Date(1714...)/`** — The CLI normalizes Bing's legacy WCF dates to ISO-8601 in its output; if you see the raw form you're reading ssl.bing.com directly rather than through this CLI.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mcp-server-bing-webmaster**](https://github.com/isiahw1/mcp-server-bing-webmaster) — Python
- [**bing-webmaster-tools (merj)**](https://github.com/merj/bing-webmaster-tools) — Python
- [**bing_webmaster_cli**](https://github.com/NmadeleiDev/bing_webmaster_cli) — Go
- [**bing-webmaster-api (seo-meow)**](https://github.com/seo-meow/bing-webmaster-api) — Rust
- [**bing-webmaster-api (webjeyros)**](https://github.com/webjeyros/bing-webmaster-api) — PHP
- [**bing-webmastertools-python (charlesnagy)**](https://github.com/charlesnagy/bing-webmastertools-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
