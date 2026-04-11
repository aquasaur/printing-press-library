# Weather Goat CLI

Weather GOAT — forecasts, alerts, air quality, and activity verdicts powered by Open-Meteo and NWS

Learn more at [Weather](https://open-meteo.com).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/other/weather-goat/cmd/weather-goat-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
weather-goat-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
weather-goat-pp-cli air-quality list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`weather`** — Current conditions + today's forecast + active alerts in one glance — zero args after setup
- **`go`** — GO/CAUTION/STOP verdicts for walk, bike, hike, commute, and drive based on weather thresholds
- **`normal`** — Compare today's weather to the 30-year historical average — see if this temperature is unusual
- **`compare`** — Side-by-side forecast for two locations to help choose where to go
- **`breathe`** — Combined AQI + pollen + UV with outdoor exercise recommendation

## Usage

<!-- HELP_OUTPUT -->

## Commands

### air-quality

Get air quality data — AQI, pollen, PM2.5, UV index

- **`weather-goat-pp-cli air-quality get`** - Get current air quality index, PM2.5, PM10, pollen levels, and UV index

### forecast

Get weather forecasts — current, hourly, and daily

- **`weather-goat-pp-cli forecast hourly`** - Get hourly forecast for the next 48 hours
- **`weather-goat-pp-cli forecast now`** - Get current weather conditions and today's forecast

### geocoding

Resolve location names to coordinates

- **`weather-goat-pp-cli geocoding search`** - Search for a location by name and get coordinates

### history

Get historical weather data (1940-present)

- **`weather-goat-pp-cli history get`** - Get historical weather data for any date back to 1940


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
weather-goat-pp-cli air-quality list

# JSON for scripting and agents
weather-goat-pp-cli air-quality list --json

# Filter to specific fields
weather-goat-pp-cli air-quality list --json --select id,name,status

# Dry run — show the request without sending
weather-goat-pp-cli air-quality list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
weather-goat-pp-cli air-quality list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | weather-goat-pp-cli <resource> create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add weather weather-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "weather": {
      "command": "weather-pp-mcp"
    }
  }
}
```

## Cookbook

Common workflows and recipes:

```bash
# List resources as JSON for scripting
weather-goat-pp-cli air-quality list --json

# Filter to specific fields
weather-goat-pp-cli air-quality list --json --select id,name,status

# Dry run to preview the request
weather-goat-pp-cli air-quality list --dry-run

# Sync data locally for offline search
weather-goat-pp-cli sync

# Search synced data
weather-goat-pp-cli search "query"

# Export for backup
weather-goat-pp-cli export --format jsonl > backup.jsonl
```

## Health Check

```bash
weather-goat-pp-cli doctor
```

<!-- DOCTOR_OUTPUT -->

## Configuration

Config file: `~/.config/weather-goat-pp-cli/config.toml`

Environment variables:

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `weather-goat-pp-cli doctor` to check credentials

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- If persistent, wait a few minutes and try again

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wttr.in**](https://github.com/chubin/wttr.in) — Python
- [**open-meteo-mcp**](https://github.com/cmer81/open-meteo-mcp) — TypeScript
- [**wthrr**](https://github.com/ttytm/wthrr-the-weathercrab) — Rust

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
