---
name: pp-dub
description: "Use this skill whenever the user wants to create, manage, or analyze short links; track click analytics; manage domains; generate QR codes; run affiliate/partner programs; handle commissions or payouts; or work with the Dub link-management platform. Dub CLI covering links, analytics, domains, QR codes, folders, tags, partners, bounties, commissions, and payouts. Requires a Dub API token. Triggers on phrasings like 'create a short link for this URL', 'how many clicks did my campaign get last week', 'generate a QR code for this link', 'pay out partners for March', 'which links are my top performers'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["dub-pp-cli"],"env":["DUB_API_KEY"]},"primaryEnv":"DUB_API_KEY","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest","bins":["dub-pp-cli"],"label":"Install via go install"}]}}'
---

# Dub — Printing Press CLI

Create short links, track analytics, manage domains, and run affiliate/partner programs via the Dub API. Full coverage of links, analytics, domains, QR codes, folders, tags, partners, bounties, commissions, and payouts. Agent-native output on every command; offline SQLite sync for bulk analytics.

## When to Use This CLI

Reach for this when the user wants to:

- Create or update short links (marketing campaigns, affiliate tracking, one-off redirects)
- Analyze link performance (clicks, unique visitors, geographic breakdown, referrer data)
- Manage an affiliate or partner program (bounties, commissions, payouts, leaderboards)
- Bulk-ship links for a launch or campaign
- Generate QR codes for print / physical campaigns

Don't reach for this if the user just needs a one-off random short link without tracking — a no-auth service is faster. Dub earns its complexity when the user actually wants the analytics and partner-program layers.

## Unique Capabilities

These commands reward the combination of link management + partner ops + analytics.

### Bulk operations

- **`links bulk-create`** / **`bulk-update`** / **`bulk-delete`** — Atomic multi-link operations. Handles transactional rollback on partial failure.

  _One command to ship 500 links for a campaign. Dub's API has bulk endpoints; this surfaces them as first-class CLI commands._

- **`links duplicates`** / **`stale`** — Find duplicate destinations or links nobody's clicked in N days. Inventory-hygiene tools that don't exist in the web dashboard.

### Analytics slicing

- **`analytics`** — Multi-dimensional analytics with flexible `--group-by` (country, device, referrer, tag, folder, date).

- **`events`** — Raw click event stream. Pipe into jq for custom slicing.

- **`tags analytics`** — Aggregate analytics for every link with a given tag.

- **`domains report`** — Per-domain performance summary.

- **`customers journey`** — Customer-journey analytics: what path did a unique visitor take through your links.

- **`partners leaderboard`** — Top-performing partners by conversions.

- **`funnel`** — Conversion funnel from click → signup → purchase (when conversion tracking is enabled).

### Partner program ops

- **`bounties`** — Create, list, approve, reject bounty submissions.

- **`commissions`** — Track commissions per partner.

- **`payouts`** — Run payouts to partners (preview with `--dry-run`).

- **`partners`** — Partner CRUD + ban/unban.

- **`campaigns`** — Campaign management (groups partners + links + bounties).

### Integration utility

- **`qr`** — Generate QR codes for any short link (PNG or SVG).

- **`track`** — Ingest custom conversion events from external sources.

- **`folders`** / **`tags`** — Organizational primitives for large link portfolios.

- **`search`** — Full-text search across synced data.

## Command Reference

Links:

- `dub-pp-cli links` — List / get / update
- `dub-pp-cli links create` — Create a single link
- `dub-pp-cli links bulk-create | bulk-update | bulk-delete` — Bulk ops
- `dub-pp-cli links duplicates` / `stale` — Hygiene
- `dub-pp-cli qr <linkId>` — QR code generation

Analytics:

- `dub-pp-cli analytics [--group-by …]` — Multi-dim analytics
- `dub-pp-cli events` — Raw event stream
- `dub-pp-cli funnel` — Conversion funnel
- `dub-pp-cli tags analytics` — Tag-scoped analytics
- `dub-pp-cli domains report` — Per-domain report
- `dub-pp-cli customers journey` — Per-customer path

Partners / Commissions:

- `dub-pp-cli partners [leaderboard|ban|approve]`
- `dub-pp-cli bounties [list|approve-bounty|ban]`
- `dub-pp-cli commissions`
- `dub-pp-cli payouts [--dry-run]`
- `dub-pp-cli campaigns`

Organization:

- `dub-pp-cli folders` — Folder CRUD
- `dub-pp-cli tags` — Tag CRUD
- `dub-pp-cli domains` — Domain management

Auth / utility:

- `dub-pp-cli auth set-token <DUB_API_KEY>`
- `dub-pp-cli tokens` — Manage API tokens (admin)
- `dub-pp-cli sync` / `export` / `import` / `archive` — Local store
- `dub-pp-cli search <query>` — Full-text search
- `dub-pp-cli doctor` — Verify
- `dub-pp-cli tail` — Live event log

## Recipes

### Ship 200 links for a launch

```bash
cat launch-links.jsonl | dub-pp-cli links bulk-create --domain dub.sh --agent
```

One atomic request, transactional rollback if any link fails validation. Much faster than 200 individual creates.

### Weekly campaign analytics

```bash
dub-pp-cli analytics --interval 7d --group-by country --agent
dub-pp-cli analytics --interval 7d --group-by device --agent
dub-pp-cli tags analytics --tag "q4-launch" --agent
```

Three slices of the same week's data: geographic, device, and campaign-tagged. Compose into a weekly report.

### Partner program month-end

```bash
dub-pp-cli commissions --interval 30d --agent          # what's owed to whom
dub-pp-cli payouts --interval 30d --dry-run --agent    # preview
dub-pp-cli payouts --interval 30d --yes --agent        # execute
dub-pp-cli partners leaderboard --interval 30d --agent # ranking
```

Always preview with `--dry-run` before running payouts. The leaderboard call at the end gives context for the next month's partner outreach.

### Find stale links eating domain budget

```bash
dub-pp-cli links stale --days 90 --agent
dub-pp-cli links bulk-delete --ids "$(dub-pp-cli links stale --days 90 --agent | jq -r '.[].id' | paste -sd, -)" --dry-run
```

Identify links nobody's clicked in 90 days, then preview a bulk delete. Domain-hygiene task that's hard to do in the web UI.

## Auth Setup

Requires a Dub API token.

```bash
# Get a token: https://app.dub.co/settings/tokens
export DUB_API_KEY="dub_xxx"
dub-pp-cli auth set-token "$DUB_API_KEY"
dub-pp-cli doctor
```

Optional:
- `DUB_BASE_URL` — override API base (for self-hosted or region-specific endpoints)
- `DUB_WORKSPACE` — default workspace slug

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes`. Universal flags: `--select`, `--dry-run`, `--rate-limit N`, `--no-cache`.

Flag glossary:
- `--data-source auto|live|local` — read from live API, local sync, or let the CLI decide
- `--interval <duration>` — analytics time window (7d, 30d, etc.)
- `--group-by <field>` — analytics aggregation dimension

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (link, partner, domain) |
| 4 | Auth required |
| 5 | API error |
| 7 | Rate limited |
| 10 | Config error |

## Installation

### CLI

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest
dub-pp-cli auth set-token YOUR_DUB_API_KEY
dub-pp-cli doctor
```

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-mcp@latest
claude mcp add -e DUB_API_KEY=<key> dub-pp-mcp -- dub-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `dub-pp-cli --help`
2. **`install`** → CLI; **`install mcp`** → MCP
3. **Anything else** → check `which dub-pp-cli` (install if missing), verify `DUB_API_KEY` is set (prompt for setup if not), route by intent: short-link creation → `links create`; analytics lookup → `analytics` or `tags analytics`; partner ops → `partners` / `commissions` / `payouts`. Run with `--agent`.
