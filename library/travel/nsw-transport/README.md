# Transport for NSW Open Data CLI

**Every NSW transport, traffic, and fuel feed in one CLI, plus cross-source commands no single API offers.**

Wraps five scattered NSW open-data APIs — GTFS-Realtime transit feeds, the Trip Planner, Live Traffic hazards, Live Traffic cameras, and FuelCheck — behind one binary with `--json`, typed exit codes, and a local SQLite store backing `search`/`sql` and `fuel-drift`. On top of matching every Home Assistant integration and the Raycast extension, it adds commands those tools can't: `commute` (is my route blocked, checking transit + roads + line alerts together), `fuel-stop` (cheapest fuel near a named place), `whereis` (live vehicle positions on a route, ranked by distance from your stop), and `station` (everything about a stop in one call). First tool — and first agent surface — to treat Sydney location intelligence as one thing.

Printed by [@aquasaur](https://github.com/aquasaur) (Cameron Bailey).

## Install

The recommended path installs both the `nsw-transport-pp-cli` binary and the `pp-nsw-transport` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install nsw-transport
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install nsw-transport --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nsw-transport-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nsw-transport --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nsw-transport --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-nsw-transport skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-nsw-transport. The skill defines how its required CLI can be installed.
```

## Authentication

Most commands use a free Transport Open Data Hub API key in the `NSW_OPENDATA_API_KEY` env var (register an app at opendata.transport.nsw.gov.au and subscribe to the GTFS-Realtime, Trip Planner, and Live Traffic products). The `fuel ...` commands additionally need free FuelCheck credentials (`NSW_FUELCHECK_API_KEY` + `NSW_FUELCHECK_API_SECRET` from api.nsw.gov.au) — the CLI does the OAuth2 handshake and caches the token; without them the fuel commands fail with an actionable message and everything else still works.

## Quick Start

```bash
# confirm NSW_OPENDATA_API_KEY is set and the Open Data Hub is reachable
nsw-transport-pp-cli doctor


# find a stop ID to use — Central Station is 10101100
nsw-transport-pp-cli trip stops "Central"


# next five departures from Central, with realtime estimates
nsw-transport-pp-cli trip departures 10101100 --count 5


# live positions of every Sydney Trains service right now
nsw-transport-pp-cli realtime vehicles sydneytrains --json


# populate the local store so `search`/`sql` work offline (the cross-source commands fetch live)
nsw-transport-pp-cli sync


# cheapest E10 within 5 km of Central
nsw-transport-pp-cli fuel-stop E10 --near "Central" --radius 5 --sort price

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-source location intelligence
- **`commute`** — Plan a journey A→B and immediately see open road hazards near the journey's endpoints plus active alerts on the lines it uses — one verdict on whether your route is clear.

  _When a user asks 'is my route clear right now', this is the one call that checks the journey, nearby road hazards, and the lines it uses together._

  ```bash
  nsw-transport-pp-cli commute "Marrickville" "Central" --depart 0815 --agent
  ```
- **`fuel-stop`** — Find the cheapest station for a fuel type within N km of a named stop/POI or a planned trip's destination, ranked by price.

  _Answers 'where should I fill up near X' without the user knowing any coordinates._

  ```bash
  nsw-transport-pp-cli fuel-stop E10 --near "Central" --radius 5 --sort price --agent
  ```
- **`station`** — Everything about one stop in a single call: next departures, line alerts, the nearest traffic cameras, and the cheapest nearby fuel.

  _When an agent is asked about a specific stop, this is the one call that returns the full picture._

  ```bash
  nsw-transport-pp-cli station 2204135 --agent
  ```

### Realtime tracking
- **`whereis`** — Show live GTFS-Realtime vehicle positions on a named route or line, ranked by distance from a chosen stop.

  _Direct answer to 'where is the 333 right now and how far from my stop' — the top user workflow._

  ```bash
  nsw-transport-pp-cli whereis route 333 --near 203311 --agent
  ```
- **`disruptions`** — Today's disruptions across every transport mode — GTFS-RT service alerts unioned with Trip Planner service-status messages, deduped and severity-sorted.

  _One call answers 'what's broken on the network right now' instead of polling per-mode feeds._

  ```bash
  nsw-transport-pp-cli disruptions --all --agent
  ```

### Roads & traffic
- **`cameras-near`** — List traffic cameras within N km of a stop or a lat/lng, with their JPG hrefs and optional download.

  _Lets a user eyeball the road near a place without scrolling hundreds of cameras._

  ```bash
  nsw-transport-pp-cli cameras-near 203311 --radius 3 --agent
  ```
- **`fuel-drift`** — Show the price change per station for a fuel type since the last snapshot (recorded by `refresh` / `fuel prices`).

  _Surfaces the actionable signal (prices moved at specific stations) that the raw FuelCheck price endpoints can't._

  ```bash
  nsw-transport-pp-cli fuel-drift E10 --brand "7-Eleven" --agent
  ```

## Usage

Run `nsw-transport-pp-cli --help` for the full command reference and flag list.

## Commands

### cameras

Live Traffic NSW road cameras

- **`nsw-transport-pp-cli cameras list`** - List all NSW traffic cameras (id, region, title, view, direction, image href)

### carpark

Park & Ride car-park availability

- **`nsw-transport-pp-cli carpark list`** - All Park & Ride facilities with current availability
- **`nsw-transport-pp-cli carpark show`** - Availability for one Park & Ride facility

### hazards

Live Traffic NSW road hazards — incidents, roadworks, floods, fires, alpine conditions, major events

- **`nsw-transport-pp-cli hazards list`** - List road hazards for a category and status

### trip

TfNSW Trip Planner — stop search, journey planning, departure boards, nearby stops, service status

- **`nsw-transport-pp-cli trip alerts`** - Service-status messages (planned and unplanned disruptions)
- **`nsw-transport-pp-cli trip departures`** - Real-time departure board for a stop
- **`nsw-transport-pp-cli trip nearby`** - Find stops and POIs near a coordinate
- **`nsw-transport-pp-cli trip plan`** - Plan a journey between two stops or coordinates, with leg-by-leg detail
- **`nsw-transport-pp-cli trip stops`** - Find stops, stations, wharves, light-rail stops and POIs by name


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nsw-transport-pp-cli cameras

# JSON for scripting and agents
nsw-transport-pp-cli cameras --json

# Filter to specific fields
nsw-transport-pp-cli cameras --json --select id,name,status

# Dry run — show the request without sending
nsw-transport-pp-cli cameras --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nsw-transport-pp-cli cameras --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-nsw-transport -g
```

Then invoke `/pp-nsw-transport <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add nsw-transport nsw-transport-pp-mcp -e NSW_OPENDATA_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nsw-transport-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NSW_OPENDATA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nsw-transport": {
      "command": "nsw-transport-pp-mcp",
      "env": {
        "NSW_OPENDATA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
nsw-transport-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/nsw-transport-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NSW_OPENDATA_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `nsw-transport-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NSW_OPENDATA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 503 "Your API quota or rate limit has been exceeded"** — You are polling faster than the feeds refresh. Back off to one request per ~15 seconds; the realtime feeds only update every 10–15s anyway.
- **HTTP 500 "Policy Falsified" on a realtime or live-traffic call** — The Authorization header is malformed or your app isn't subscribed to that API product. Check `NSW_OPENDATA_API_KEY` has no quotes/whitespace and subscribe to the product in the Open Data Hub portal.
- **`trip stops` returns error -2000 "stop invalid"** — Use the default search (type any). The Aug-2025 Trip Planner upgrade broke name searches with type_sf=stop — this CLI already defaults to `any`.
- **`fuel ...` commands say credentials are missing** — Set `NSW_FUELCHECK_API_KEY` and `NSW_FUELCHECK_API_SECRET` from your api.nsw.gov.au FuelCheck app. They're separate from the Open Data Hub key.
- **`fuel-drift` reports "not enough history"** — Run `nsw-transport-pp-cli refresh` (or `fuel prices`) at least twice; each records a price snapshot. The other cross-source commands fetch live and need no sync.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ha_transportnsw**](https://github.com/andystewart999/ha_transportnsw) — Python
- [**transport-nsw-opendata**](https://github.com/rycus86/transport-nsw-opendata) — Python
- [**nsw-fuel-api-client**](https://github.com/nickw444/nsw-fuel-api-client) — Python
- [**TfNSW_GTFSRDB**](https://github.com/tarasutjarittham/TfNSW_GTFSRDB) — Python
- [**PyTransportNSW**](https://github.com/Dav0815/TransportNSW) — Python
- [**sydney-bus-departures**](https://github.com/jakecoppinger/sydney-bus-departures) — TypeScript
- [**tfnsw-opal-fare-calculator**](https://github.com/jxeeno/tfnsw-opal-fare-calculator) — JavaScript
- [**ltfschoen/opendata-api**](https://github.com/ltfschoen/opendata-api) — Ruby

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
