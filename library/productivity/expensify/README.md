# Expensify CLI

**File expenses and submit reports to Expensify in one line. Every command an agent should need, with a local cache so searches stay offline.**

expensify-pp-cli turns the Expensify web app into a terminal. Log in once, and every filing/reviewing/submitting task that used to require clicking through forms becomes a single command. A local SQLite store gives you offline search, rollups, dupe detection, and missing-receipt alerts that no other Expensify tool has.

Learn more at [Expensify](https://www.expensify.com/).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/productivity/expensify/cmd/expensify-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Authentication

Two ways to authenticate: (1) `expensify auth login` opens a browser, you log in, the CLI captures your session token — works immediately for all filing/submitting commands; (2) `expensify auth set-keys` stores your Integration Server partner credentials (get them at https://www.expensify.com/tools/integrations/) — required only for export/admin commands. Most users only need option 1.

## Quick Start

```bash
# Browser-based login — captures your Expensify session
expensify-pp-cli auth login


# Pull your workspaces, expenses, and reports into a local cache
expensify-pp-cli sync


# File your first expense in one line
expensify-pp-cli expense quick "Dinner at Maya $42.50"


# Create a report and attach every un-reported expense from April
expensify-pp-cli report draft --since 2026-04-01 --title "April expenses"


# Submit for approval and block until it moves forward
expensify-pp-cli report submit <report-id> --wait

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Filing at thought speed

- **`expense quick`** — File an expense with one line: amount, merchant, and category parsed from a short prompt — no forms, no web UI.

  _When a user tells an agent 'I just expensed dinner at Maya for $42.50,' the agent should file it in one call — not walk through six fields._

  ```bash
  expensify-pp-cli expense quick "Dinner at Maya $42.50" --agent
  ```
- **`report draft`** — Create a report and auto-attach every un-reported expense from a date range in a single command.

  _End-of-month submission turns from 45 clicks into one command._

  ```bash
  expensify-pp-cli report draft --since 2026-04-01 --title "April expenses" --policy POLICY_ID_HERE
  ```
- **`expense watch`** — Daemon mode — drop a receipt PNG/JPG/PDF into a folder, it auto-files as an expense.

  _A scanner app or Siri shortcut drops files into Dropbox; this files them automatically._

  ```bash
  expensify-pp-cli expense watch ~/Receipts --policy POLICY_ID_HERE
  ```
- **`expense from-line`** — Paste a raw bank or card CSV row and the CLI extracts date/merchant/amount/currency and files the expense.

  _Reconciling AmEx? Paste a row, file an expense. No copy-paste-copy-paste._

  ```bash
  expensify-pp-cli expense from-line "2026-04-18 DOORDASH*JOES $14.25" --category Meals
  ```
- **`expense new (default behavior)`** — When filing an expense, suggests a category based on prior classifications for the same merchant.

  _Filing is faster when the category is already right 90% of the time._

  ```bash
  expensify-pp-cli expense quick "Uber $24"  # auto-suggests Transportation
  ```

### Local state that compounds

- **`damage`** — Single-glance summary: total expensed, pending, approved, paid for the current month (or a custom range).

  _Agents asked 'how much did I expense this month' get one answer in one call._

  ```bash
  expensify-pp-cli damage --month current --json
  ```
- **`expense search`** — FTS5 search over all your expenses by merchant, comment, category, or tag. Regex-friendly.

  _Agents asked 'did I expense that Starbucks last month' get an answer in one local query._

  ```bash
  expensify-pp-cli expense search "coffee" --since 2026-01-01 --json
  ```
- **`expense missing-receipts`** — Lists expenses without attached receipts so you can catch them before submitting a report.

  _Submit-report-and-get-bounced feels bad; surface missing receipts upfront._

  ```bash
  expensify-pp-cli expense missing-receipts --json
  ```
- **`expense rollup`** — Pivot-table expenses by category, tag, or merchant for any time range.

  _Build your own spending dashboard without burning API budget._

  ```bash
  expensify-pp-cli expense rollup --month 2026-04 --by category
  ```
- **`expense dupes`** — Finds expenses that look like duplicates by (merchant, amount, date±window).

  _Accidental double-file is a top AP pain point; surface it before submission._

  ```bash
  expensify-pp-cli expense dupes --window 3d --json
  ```

### Agent-native plumbing

- **`report submit --wait`** — Blocks until the report transitions past SUBMITTED, great for CI pipelines.

  _CI jobs that submit expense reports can wait for approval before moving on._

  ```bash
  expensify-pp-cli report submit REPORT_ID_HERE --wait --timeout 1h
  ```
- **`undo`** — Revert the last create/edit/submit action from the local action log.

  _Agents sometimes file the wrong amount; one command to roll it back._

  ```bash
  expensify-pp-cli undo --dry-run
  ```
- **`mcp`** — Expose every subcommand as an MCP tool so Claude Desktop and other MCP clients can drive Expensify.

  _Claude Desktop gets a single-install Expensify integration that matches this CLI's coverage exactly._

  ```bash
  expensify-pp-cli mcp --port 7021
  ```

### Admin orchestration

- **`close`** — End-of-month orchestrator: list reports in range, export with GL template, download the file, mark-as-exported atomically.

  _Finance teams close a month in one command instead of a 40-click sequence._

  ```bash
  expensify-pp-cli close --month 2026-04 --template netsuite --label "Apr close"
  ```
- **`admin policy-diff`** — Compare local YAML policy config against live API to preview changes before applying.

  _Manage chart-of-accounts as code with dry-run safety._

  ```bash
  expensify-pp-cli admin policy-diff POLICY_ID_HERE categories.yaml
  ```

## Usage

Run `expensify-pp-cli --help` for the full command reference and flag list.

## Commands

### admin

Integration Server: policy, employee, and rules admin

- **`expensify-pp-cli admin cards_list`** - List domain cards (Domain Cards Getter)
- **`expensify-pp-cli admin cards_owners`** - List card owners (Card Owner Data)
- **`expensify-pp-cli admin employee_add`** - Add an employee to a policy (Advanced Employee Updater)
- **`expensify-pp-cli admin employee_remove`** - Remove an employee from a policy
- **`expensify-pp-cli admin employee_update`** - Update an employee (Advanced Employee Updater)
- **`expensify-pp-cli admin policy_get`** - Get a policy's full config (Policy Getter)
- **`expensify-pp-cli admin policy_list`** - List all policies you admin (Policy List Getter)
- **`expensify-pp-cli admin policy_new`** - Create a new policy (Policy Creator)
- **`expensify-pp-cli admin policy_set_categories`** - Update categories for a policy from YAML
- **`expensify-pp-cli admin policy_set_fields`** - Update report fields for a policy
- **`expensify-pp-cli admin policy_set_tags`** - Update tags for a policy from YAML
- **`expensify-pp-cli admin report_set_status`** - Force a report status transition (Report Status Updater)
- **`expensify-pp-cli admin rules_new`** - Create an expense rule (Expense Rules Creator)
- **`expensify-pp-cli admin rules_update`** - Update an expense rule
- **`expensify-pp-cli admin tag_approvers_set`** - Set tag approvers (Tag Approvers Updater)

### category

Workspace categories (for expense classification)

- **`expensify-pp-cli category list`** - List categories for a workspace

### expense

Create, list, and manage personal expenses

- **`expensify-pp-cli expense attach`** - Attach or replace a receipt on an expense
- **`expensify-pp-cli expense create`** - Create a new expense
- **`expensify-pp-cli expense delete`** - Delete an expense
- **`expensify-pp-cli expense edit`** - Edit an existing expense
- **`expensify-pp-cli expense get`** - Get expense detail by transaction ID
- **`expensify-pp-cli expense list`** - List your expenses with filters

### export

Integration Server: export reports to accounting systems (admin)

- **`expensify-pp-cli export download`** - Download a previously generated export file
- **`expensify-pp-cli export run`** - Export reports via Report Exporter (Integration Server)

### me

Current user profile

- **`expensify-pp-cli me get`** - Get current user profile

### recon

Integration Server: corporate card reconciliation (admin)

- **`expensify-pp-cli recon export`** - Export reconciliation data for a domain

### report

Create, manage, and submit expense reports

- **`expensify-pp-cli report add`** - Add expenses to a report
- **`expensify-pp-cli report approve`** - Approve a report (manager action)
- **`expensify-pp-cli report comment`** - Add a comment to a report thread
- **`expensify-pp-cli report create`** - Create a new report
- **`expensify-pp-cli report delete`** - Delete a draft report
- **`expensify-pp-cli report get`** - Get report detail
- **`expensify-pp-cli report list`** - List your reports
- **`expensify-pp-cli report pay`** - Mark a report as reimbursed
- **`expensify-pp-cli report reopen`** - Reopen a submitted report back to draft
- **`expensify-pp-cli report submit`** - Submit a report for approval

### tag

Workspace tags (multi-level, for expense classification)

- **`expensify-pp-cli tag list`** - List tags for a workspace

### workspace

View workspaces (policies) you have access to

- **`expensify-pp-cli workspace get`** - Get workspace detail
- **`expensify-pp-cli workspace list`** - List workspaces accessible to your account


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
expensify-pp-cli category list

# JSON for scripting and agents
expensify-pp-cli category list --json

# Filter to specific fields
expensify-pp-cli category list --json --select id,name,status

# Dry run — show the request without sending
expensify-pp-cli category list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
expensify-pp-cli category list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | expensify-pp-cli <resource> create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add expensify expensify-pp-mcp -e EXPENSIFY_AUTH_TOKEN=<your-key>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "expensify": {
      "command": "expensify-pp-mcp",
      "env": {
        "EXPENSIFY_AUTH_TOKEN": "<your-key>"
      }
    }
  }
}
```

## Health Check

```bash
expensify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/expensify-pp-cli/config.toml`

Environment variables:
- `EXPENSIFY_AUTH_TOKEN`
- `EXPENSIFY_PARTNER_USER_ID`
- `EXPENSIFY_PARTNER_USER_SECRET`

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `expensify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EXPENSIFY_AUTH_TOKEN`

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- If persistent, wait a few minutes and try again

### API-specific

- **`403 Forbidden` on every command** — Your session expired. Run `expensify-pp-cli auth login` again.
- **`429 Too Many Requests`** — Integration Server enforces 5 req / 10s and 20 req / 60s. The CLI backs off automatically; if you see this, wait 60 seconds before retrying.
- **`export run` asks for partner credentials** — Export commands use the Integration Server, not your session. Run `expensify-pp-cli auth set-keys` with credentials from https://www.expensify.com/tools/integrations/.
- **`expense quick` can't parse my input** — Format: `<description> <merchant> $<amount>`. Example: `"Dinner at Maya $42.50"`. Use `--amount/--merchant/--category` flags for explicit control.
- **Policy/category autocomplete is empty** — Run `expensify-pp-cli sync` to refresh the local cache.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**justexpenseit**](https://github.com/meyskens/justexpenseit) — Go (1 stars)
- [**primrose-mcp-expensify**](https://github.com/primrose-mcp/primrose-mcp-expensify) — TypeScript
- [**expensify-mcp-http**](https://github.com/agenticledger/expensify-mcp-http) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
