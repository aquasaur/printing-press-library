# Dub CLI

**Every Dub operation, plus the local-store features no Dub tool — official or community — has ever shipped.**

Manage Dub short links, analytics, partner programs, and conversion tracking from the terminal. A local SQLite store unlocks dead-link detection, performance drift, conversion funnels, partner leaderboards, and cross-tag analytics — without burning the rate-limit budget. Every command is agent-native: --json, --select, --dry-run, typed exit codes.

Learn more at [Dub](https://dub.co/support).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Authentication

Dub uses workspace-scoped API keys. Get one at https://app.dub.co/settings/tokens and export it as `DUB_TOKEN` (the CLI also accepts `DUB_API_KEY` as an alias for parity with Dub's own SDK examples). `dub-pp-cli doctor` confirms the key reaches the API.

## Quick Start

```bash
# Workspace key from app.dub.co/settings/tokens
export DUB_TOKEN=dub_yourkey


# Confirm auth and reach the API
dub-pp-cli doctor


# Pull links, tags, folders, domains, partners, customers into the local store
dub-pp-cli sync --full


# Inspect links from the local cache
dub-pp-cli links get --json --select id,domain,key,clicks


# See which tags drive the most clicks, leads, sales
dub-pp-cli campaigns --interval 30d


# Find dead links worth cleaning up
dub-pp-cli links stale --days 90


# Mint a short link
dub-pp-cli links create --url https://example.com --domain dub.sh

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`links stale`** — Find archived, expired, or zero-traffic links across the workspace before they pile up. Joins local link metadata with analytics aggregates the API doesn't expose together.

  _Pick up cleanup work without round-tripping every link's analytics endpoint individually._

  ```bash
  dub-pp-cli links stale --days 90 --json
  ```
- **`links drift`** — Detect links whose click rate dropped more than threshold percent week-over-week. Catches dying campaigns before reporting deadlines.

  _Pre-emptive alerting on silent campaign collapse — catches issues before someone notices traffic is gone._

  ```bash
  dub-pp-cli links drift --window 7d --threshold 30 --json
  ```
- **`campaigns`** — Performance dashboard aggregated by tag — clicks, leads, sales rolled up across every link wearing each tag.

  _Campaign-level view of what's actually working without manual rollup spreadsheets._

  ```bash
  dub-pp-cli campaigns --interval 30d --json
  ```
- **`funnel`** — Click-to-lead-to-sale conversion rates per link or campaign. Surfaces where prospects drop off in the funnel.

  _Attribution debugging without exporting to a BI tool._

  ```bash
  dub-pp-cli funnel --link link_xyz --json
  ```
- **`customers journey`** — See every link a customer clicked, when they became a lead, and when they purchased — in one timeline.

  _Single-pane-of-glass attribution for one customer instead of stitching four endpoints._

  ```bash
  dub-pp-cli customers journey --customer cus_xyz --json
  ```
- **`partners leaderboard`** — Rank partners by commission earned, conversion rate, and clicks generated. Find your top performers and dormant partners.

  _Partner-program health at a glance without four separate API calls._

  ```bash
  dub-pp-cli partners leaderboard --by commission --interval 30d --json
  ```

### Hygiene and safety

- **`links duplicates`** — Find every link in the workspace pointing to the same destination URL. Surfaces accidental duplicates and consolidation candidates.

  _Catches accidental duplicates that fragment analytics and waste short-link slugs._

  ```bash
  dub-pp-cli links duplicates --json --select url,count,short_links
  ```
- **`links lint`** — Audit short-key slugs for lookalike collisions, reserved-word violations, and brand-conflict hazards across domains.

  _Catches typo-shaped redirects and ambiguous aliases before they ship to a campaign._

  ```bash
  dub-pp-cli links lint --json
  ```
- **`links rewrite`** — Show every link that would change and the exact patch BEFORE sending. Mass UTM or domain migrations with dry-run safety.

  _Campaign-wide rewrites without blast-radius mistakes._

  ```bash
  dub-pp-cli links rewrite --match 'utm_source=oldsrc' --replace 'utm_source=newsrc' --dry-run
  ```
- **`partners audit-commissions`** — Reconcile partners, commissions, bounties, and payouts to flag stale rates, missing payouts, and expired bounties still earning.

  _Partner-program audit that would otherwise be four API calls plus a spreadsheet._

  ```bash
  dub-pp-cli partners audit-commissions --json
  ```
- **`domains report`** — Per-domain link count and click distribution. Surfaces over- and under-used custom domains.

  _See which custom domains are pulling weight before renewal time._

  ```bash
  dub-pp-cli domains report --json
  ```
- **`health`** — Cross-resource Monday-morning report: rate-limit headroom, expired-but-active links, dead destination URLs, unverified domains, dormant tags.

  _One command instead of five to triage workspace health._

  ```bash
  dub-pp-cli health --json
  ```

### Agent-native plumbing

- **`since`** — What happened in the last N hours? Created, updated, deleted links plus partner approvals and top-clicked entities.

  _Powers agent 'what changed' flows without polling a dozen list endpoints._

  ```bash
  dub-pp-cli since 24h --json
  ```
- **`tail`** — Stream live changes by polling the API at regular intervals. Watch new clicks, leads, and sales as they happen.

  _Watch a campaign launch in real time without refreshing the dashboard._

  ```bash
  dub-pp-cli tail --interval 10s
  ```

## Usage

Run `dub-pp-cli --help` for the full command reference and flag list.

## Commands

### analytics

Manage analytics

- **`dub-pp-cli analytics retrieve`** - Retrieve analytics for a link, a domain, or the authenticated workspace.

### bounties

Manage bounties


### commissions

Manage commissions

- **`dub-pp-cli commissions bulk-update`** - Bulk update commissions
- **`dub-pp-cli commissions list`** - List all commissions
- **`dub-pp-cli commissions update`** - Update a commission

### customers

Manage customers

- **`dub-pp-cli customers delete`** - Delete a customer
- **`dub-pp-cli customers get`** - List all customers
- **`dub-pp-cli customers get-id`** - Retrieve a customer
- **`dub-pp-cli customers update`** - Update a customer

### domains

Manage domains

- **`dub-pp-cli domains check-status`** - Check the availability of one or more domains
- **`dub-pp-cli domains create`** - Create a domain
- **`dub-pp-cli domains delete`** - Delete a domain
- **`dub-pp-cli domains list`** - List all domains
- **`dub-pp-cli domains register`** - Register a domain
- **`dub-pp-cli domains update`** - Update a domain

### events

Manage events

- **`dub-pp-cli events list`** - List all events

### folders

Manage folders

- **`dub-pp-cli folders create`** - Create a folder
- **`dub-pp-cli folders delete`** - Delete a folder
- **`dub-pp-cli folders list`** - List all folders
- **`dub-pp-cli folders update`** - Update a folder

### links

Manage links

- **`dub-pp-cli links bulk-create`** - Bulk create links
- **`dub-pp-cli links bulk-delete`** - Bulk delete links
- **`dub-pp-cli links bulk-update`** - Bulk update links
- **`dub-pp-cli links create`** - Create a link
- **`dub-pp-cli links delete`** - Delete a link
- **`dub-pp-cli links get`** - List all links
- **`dub-pp-cli links get-count`** - Retrieve links count
- **`dub-pp-cli links get-info`** - Retrieve a link
- **`dub-pp-cli links update`** - Update a link
- **`dub-pp-cli links upsert`** - Upsert a link

### partners

Manage partners

- **`dub-pp-cli partners approve`** - Approve a partner application
- **`dub-pp-cli partners ban`** - Ban a partner
- **`dub-pp-cli partners create`** - Create or update a partner
- **`dub-pp-cli partners create-link`** - Create a link for a partner
- **`dub-pp-cli partners deactivate`** - Deactivate a partner
- **`dub-pp-cli partners list`** - List all partners
- **`dub-pp-cli partners list-applications`** - List all pending partner applications
- **`dub-pp-cli partners reject`** - Reject a partner application
- **`dub-pp-cli partners retrieve-analytics`** - Retrieve analytics for a partner
- **`dub-pp-cli partners retrieve-links`** - Retrieve a partner's links.
- **`dub-pp-cli partners upsert-link`** - Upsert a link for a partner

### payouts

Manage payouts

- **`dub-pp-cli payouts list`** - List all payouts

### qr

Manage qr

- **`dub-pp-cli qr get-qrcode`** - Retrieve a QR code

### tags

Manage tags

- **`dub-pp-cli tags create`** - Create a tag
- **`dub-pp-cli tags delete`** - Delete a tag
- **`dub-pp-cli tags get`** - List all tags
- **`dub-pp-cli tags update`** - Update a tag

### tokens

Manage tokens

- **`dub-pp-cli tokens create-referrals-embed`** - Create a referrals embed token

### track

Manage track

- **`dub-pp-cli track lead`** - Track a lead
- **`dub-pp-cli track open`** - Track a deep link open event
- **`dub-pp-cli track sale`** - Track a sale


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dub-pp-cli commissions list

# JSON for scripting and agents
dub-pp-cli commissions list --json

# Filter to specific fields
dub-pp-cli commissions list --json --select id,name,status

# Dry run — show the request without sending
dub-pp-cli commissions list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dub-pp-cli commissions list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add dub dub-pp-mcp -e DUB_TOKEN=<your-token>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dub": {
      "command": "dub-pp-mcp",
      "env": {
        "DUB_TOKEN": "<your-key>"
      }
    }
  }
}
```

## Health Check

```bash
dub-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dub-pp-cli/config.toml`

Environment variables:
- `DUB_TOKEN` — workspace API key (primary)
- `DUB_API_KEY` — alias for `DUB_TOKEN` (matches Dub's SDK example name)
- `DUB_BASE_URL` — override the API base URL for self-hosted Dub instances
- `DUB_FEEDBACK_ENDPOINT` — destination for opt-in upstream feedback (see `dub-pp-cli feedback --help`)
- `DUB_FEEDBACK_AUTO_SEND` — when `1`, send feedback automatically without confirmation

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dub-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DUB_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized — Missing Authorization header** — Run `dub-pp-cli doctor` to confirm `DUB_TOKEN` (or `DUB_API_KEY`) is set; refresh at https://app.dub.co/settings/tokens
- **429 Too Many Requests on analytics** — Dub enforces per-second sub-limits on /analytics. The CLI honors Retry-After. Cache via `dub-pp-cli sync` and read from the local store.
- **Local store seems stale** — Run `dub-pp-cli sync --full` to rebuild from the API.
- **Self-hosted Dub instance** — Override the base URL with DUB_BASE_URL=https://api.your-instance.com.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dubinc/dub-ts**](https://github.com/dubinc/dub-ts) — TypeScript (62 stars)
- [**sujjeee/dubco**](https://github.com/sujjeee/dubco) — TypeScript (24 stars)
- [**dubinc/dub-go**](https://github.com/dubinc/dub-go) — Go (14 stars)
- [**dubinc/dub-python**](https://github.com/dubinc/dub-python) — Python (8 stars)
- [**gitmaxd/dubco-mcp-server-npm**](https://github.com/gitmaxd/dubco-mcp-server-npm) — JavaScript (7 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
