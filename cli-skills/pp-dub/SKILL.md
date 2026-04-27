---
name: pp-dub
description: "Every Dub operation, plus the local-store features no Dub tool — official or community — has ever shipped. Trigger phrases: `shorten this URL with dub`, `list my dub links`, `audit dub partner commissions`, `find dead dub links`, `check dub campaign performance`, `use dub`, `run dub`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["dub-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest","bins":["dub-pp-cli"],"label":"Install via go install"}]}}'
---

# Dub — Printing Press CLI

Manage Dub short links, analytics, partner programs, and conversion tracking from the terminal. A local SQLite store unlocks dead-link detection, performance drift, conversion funnels, partner leaderboards, and cross-tag analytics — without burning the rate-limit budget. Every command is agent-native: --json, --select, --dry-run, typed exit codes.

## When to Use This CLI

Reach for this CLI when you need to audit, bulk-modify, or cross-analyze Dub assets in ways the web UI doesn't support. Especially good for campaign-wide UTM rewrites with --dry-run preview, partner-program audits, dead-link cleanup, conversion funnel attribution, and analytics queries an agent wants to compose with --json + --select.

## Unique Capabilities

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

## Command Reference

**analytics** — Manage analytics

- `dub-pp-cli analytics retrieve` — Retrieve analytics for a link, a domain, or the authenticated workspace.

**bounties** — Manage bounties


**commissions** — Manage commissions

- `dub-pp-cli commissions bulk-update` — Bulk update commissions
- `dub-pp-cli commissions list` — List all commissions
- `dub-pp-cli commissions update` — Update a commission

**customers** — Manage customers

- `dub-pp-cli customers delete` — Delete a customer
- `dub-pp-cli customers get` — List all customers
- `dub-pp-cli customers get-id` — Retrieve a customer
- `dub-pp-cli customers update` — Update a customer

**domains** — Manage domains

- `dub-pp-cli domains check-status` — Check the availability of one or more domains
- `dub-pp-cli domains create` — Create a domain
- `dub-pp-cli domains delete` — Delete a domain
- `dub-pp-cli domains list` — List all domains
- `dub-pp-cli domains register` — Register a domain
- `dub-pp-cli domains update` — Update a domain

**events** — Manage events

- `dub-pp-cli events list` — List all events

**folders** — Manage folders

- `dub-pp-cli folders create` — Create a folder
- `dub-pp-cli folders delete` — Delete a folder
- `dub-pp-cli folders list` — List all folders
- `dub-pp-cli folders update` — Update a folder

**links** — Manage links

- `dub-pp-cli links bulk-create` — Bulk create links
- `dub-pp-cli links bulk-delete` — Bulk delete links
- `dub-pp-cli links bulk-update` — Bulk update links
- `dub-pp-cli links create` — Create a link
- `dub-pp-cli links delete` — Delete a link
- `dub-pp-cli links get` — List all links
- `dub-pp-cli links get-count` — Retrieve links count
- `dub-pp-cli links get-info` — Retrieve a link
- `dub-pp-cli links update` — Update a link
- `dub-pp-cli links upsert` — Upsert a link

**partners** — Manage partners

- `dub-pp-cli partners approve` — Approve a partner application
- `dub-pp-cli partners ban` — Ban a partner
- `dub-pp-cli partners create` — Create or update a partner
- `dub-pp-cli partners create-link` — Create a link for a partner
- `dub-pp-cli partners deactivate` — Deactivate a partner
- `dub-pp-cli partners list` — List all partners
- `dub-pp-cli partners list-applications` — List all pending partner applications
- `dub-pp-cli partners reject` — Reject a partner application
- `dub-pp-cli partners retrieve-analytics` — Retrieve analytics for a partner
- `dub-pp-cli partners retrieve-links` — Retrieve a partner's links.
- `dub-pp-cli partners upsert-link` — Upsert a link for a partner

**payouts** — Manage payouts

- `dub-pp-cli payouts list` — List all payouts

**qr** — Manage qr

- `dub-pp-cli qr` — Retrieve a QR code (leaf shortcut for the `qr/get-qrcode` operation)

**tags** — Manage tags

- `dub-pp-cli tags create` — Create a tag
- `dub-pp-cli tags delete` — Delete a tag
- `dub-pp-cli tags get` — List all tags
- `dub-pp-cli tags update` — Update a tag

**tokens** — Manage tokens

- `dub-pp-cli tokens` — Create a referrals embed token (leaf shortcut for the `tokens/embed/referrals/links` operation)

**track** — Manage track

- `dub-pp-cli track lead` — Track a lead
- `dub-pp-cli track open` — Track a deep link open event
- `dub-pp-cli track sale` — Track a sale


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dub-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find links nobody clicks

```bash
dub-pp-cli links stale --days 90 --json --select id,domain,key,url,clicks
```

Surfaces archive candidates from the local store with no extra API calls.

### Audit campaign drift

```bash
dub-pp-cli links drift --window 7d --threshold 30 --json
```

Flags any link whose week-over-week click rate dropped by 30% or more.

### Top campaigns by tag

```bash
dub-pp-cli campaigns --interval 30d --agent --select tag,total_clicks,total_leads,total_sales
```

Joins local taxonomy with analytics and narrows agent context to the columns it needs.

### Dry-run a mass UTM rewrite

```bash
dub-pp-cli links rewrite --match 'utm_source=oldsrc' --replace 'utm_source=newsrc' --dry-run
```

See every link that would change before sending the bulk patch.

### Workspace Monday-morning health

```bash
dub-pp-cli health --json
```

One command to audit rate-limit headroom, dead destinations, and verification state.

### Single-customer funnel timeline

```bash
dub-pp-cli customers journey --customer cus_xyz --json
```

Every click, lead, and sale for one customer in chronological order.

## Auth Setup

Dub uses workspace-scoped API keys. Get one at https://app.dub.co/settings/tokens and export it as `DUB_TOKEN` (or `DUB_API_KEY` — both names are accepted). The CLI checks for the key on every command and `dub-pp-cli doctor` confirms it reaches the API. For self-hosted Dub, override the base URL with `DUB_BASE_URL=https://api.your-instance.com`.

Run `dub-pp-cli doctor` to verify setup.

## When NOT to use this CLI

Anti-triggers — pick a different tool when the user asks for:

- **Generic URL shortening** that isn't tied to a Dub workspace (try `bit.ly`, `is.gd`, or another short-link service)
- **Non-Dub link analytics** (Google Analytics, Mixpanel, Amplitude)
- **Posting to social media** with the short link (this CLI doesn't publish — it just creates and tracks)
- **Customer-data exports for non-Dub systems** (Stripe, HubSpot, Segment have their own CLIs)
- **Email or SMS delivery of links** (delivery is a separate concern)
- **Real-time webhooks or webhook delivery introspection** (Dub manages webhooks via the dashboard, not this CLI)

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dub-pp-cli commissions list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
dub-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dub-pp-cli feedback --stdin < notes.txt
dub-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.dub-pp-cli/feedback.jsonl`. They are never POSTed unless `DUB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DUB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dub-pp-cli profile save briefing --json
dub-pp-cli --profile briefing commissions list
dub-pp-cli profile list --json
dub-pp-cli profile show briefing
dub-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dub-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-cli@main
   ```
3. Verify: `dub-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/marketing/dub/cmd/dub-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dub-pp-mcp -- dub-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dub-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dub-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dub-pp-cli <command> --help`.
