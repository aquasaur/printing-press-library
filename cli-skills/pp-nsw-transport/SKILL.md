---
name: pp-nsw-transport
description: "Every NSW transport, traffic, and fuel feed in one CLI, plus cross-source commands no single API offers. Trigger phrases: `next train from Central`, `plan a trip from Marrickville to the city`, `where is the 333 bus`, `is my commute blocked`, `cheapest E10 near me`, `live traffic incidents in Sydney`, `use nsw-transport`, `run nsw-transport`."
author: "Cameron Bailey"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nsw-transport-pp-cli
---

# Transport for NSW Open Data — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nsw-transport-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install nsw-transport --cli-only
   ```
2. Verify: `nsw-transport-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps five scattered NSW open-data APIs — GTFS-Realtime transit feeds, the Trip Planner, Live Traffic hazards, Live Traffic cameras, and FuelCheck — behind one binary with `--json`, typed exit codes, and a local SQLite store backing `search`/`sql` and `fuel-drift`. On top of matching every Home Assistant integration and the Raycast extension, it adds commands those tools can't: `commute` (is my route blocked, checking transit + roads + line alerts together), `fuel-stop` (cheapest fuel near a named place), `whereis` (live vehicle positions on a route, ranked by distance from your stop), and `station` (everything about a stop in one call). First tool — and first agent surface — to treat Sydney location intelligence as one thing.

## When to Use This CLI

Reach for this CLI for any Sydney/NSW location question that touches public transport, road traffic, traffic cameras, or fuel prices — next departures, journey planning, where a bus is, whether a route is blocked, what's disrupted on the network, or the cheapest fuel near somewhere. It's the right choice over a generic HTTP client because it decodes the GTFS-Realtime protobuf, does the FuelCheck OAuth handshake, normalizes GeoJSON, and joins all five sources through a local store — so a question like 'cheapest E10 near Town Hall' or 'is the train to the CBD OK right now' is one call, not five.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**cameras** — Live Traffic NSW road cameras

- `nsw-transport-pp-cli cameras` — List all NSW traffic cameras (id, region, title, view, direction, image href)

**carpark** — Park & Ride car-park availability

- `nsw-transport-pp-cli carpark list` — All Park & Ride facilities with current availability
- `nsw-transport-pp-cli carpark show` — Availability for one Park & Ride facility

**hazards** — Live Traffic NSW road hazards — incidents, roadworks, floods, fires, alpine conditions, major events

- `nsw-transport-pp-cli hazards <category> <status>` — List road hazards for a category and status

**trip** — TfNSW Trip Planner — stop search, journey planning, departure boards, nearby stops, service status

- `nsw-transport-pp-cli trip alerts` — Service-status messages (planned and unplanned disruptions)
- `nsw-transport-pp-cli trip departures` — Real-time departure board for a stop
- `nsw-transport-pp-cli trip nearby` — Find stops and POIs near a coordinate
- `nsw-transport-pp-cli trip plan` — Plan a journey between two stops or coordinates, with leg-by-leg detail
- `nsw-transport-pp-cli trip stops` — Find stops, stations, wharves, light-rail stops and POIs by name


**Hand-written commands**

- `nsw-transport-pp-cli realtime vehicles` — Live GTFS-Realtime vehicle positions for a transport mode (decodes the protobuf feed)
- `nsw-transport-pp-cli realtime trips` — Live GTFS-Realtime trip updates (delays/cancellations) for a transport mode
- `nsw-transport-pp-cli realtime alerts` — Live GTFS-Realtime service alerts for a transport mode
- `nsw-transport-pp-cli realtime schedule` — Download the GTFS static timetable ZIP for a transport mode
- `nsw-transport-pp-cli fuel prices` — All current NSW fuel prices (FuelCheck), filterable by fuel type and brand
- `nsw-transport-pp-cli fuel near` — Fuel prices near a lat/lng within a radius, sorted by price or distance
- `nsw-transport-pp-cli fuel station` — Current fuel prices for one station by station code
- `nsw-transport-pp-cli fuel by-location` — Fuel prices for a named location (suburb or postcode)
- `nsw-transport-pp-cli fuel types` — List valid FuelCheck fuel-type codes
- `nsw-transport-pp-cli fuel brands` — List FuelCheck fuel brands
- `nsw-transport-pp-cli fuel trends` — Statewide or per-station fuel price trends over a period
- `nsw-transport-pp-cli commute` — Plan a journey A→B and report open road hazards near the journey's endpoints plus active alerts on the lines it uses
- `nsw-transport-pp-cli fuel-stop` — Cheapest fuel of a given type within N km of a named stop/POI or a trip destination
- `nsw-transport-pp-cli whereis` — Live vehicle positions on a named route, ranked by distance from a chosen stop
- `nsw-transport-pp-cli station` — Everything about one stop: next departures, line alerts, nearest cameras, and cheapest fuel nearby
- `nsw-transport-pp-cli cameras-near` — Traffic cameras within N km of a stop or lat/lng, with JPG hrefs and optional download
- `nsw-transport-pp-cli disruptions` — Today's disruptions across every transport mode, deduped from GTFS-RT alerts and Trip Planner service-status messages
- `nsw-transport-pp-cli fuel-drift` — Fuel price change per station for a fuel type since the last snapshot (run 'refresh' twice)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nsw-transport-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Is my morning commute clear?

```bash
nsw-transport-pp-cli commute "Marrickville" "Central" --depart 0815
```

Plans the journey and reports open road hazards within --hazard-radius km of the journey's endpoints plus active alerts on the lines it uses — one verdict.

### Where is the 333 bus?

```bash
nsw-transport-pp-cli whereis route 333 --near 203311 --json
```

Decodes the live bus vehicle-positions feed, filters to route 333, and ranks vehicles by distance from stop 203311.

### Cheapest diesel near where I'm driving to

```bash
nsw-transport-pp-cli fuel-stop DL --near "Penrith Station" --radius 8 --sort price
```

Resolves the stop name to coordinates and ranks nearby stations by diesel price using the local store.

### What's disrupted on the trains today (compact, agent-friendly)

```bash
nsw-transport-pp-cli disruptions --mode sydneytrains --agent --select alerts.header,alerts.severity,alerts.affected_lines
```

Unions the GTFS-RT alerts and Trip Planner service-status messages for trains, deduped, and narrows the deeply-nested response to just the headline, severity, and affected lines.

### Save a traffic camera image near a stop

```bash
nsw-transport-pp-cli cameras-near 203311 --radius 3 --save ./cam.jpg
```

Finds the nearest camera to the stop and downloads its current JPEG (cameras refresh roughly every 15 seconds).

## Auth Setup

Most commands use a free Transport Open Data Hub API key in the `NSW_OPENDATA_API_KEY` env var (register an app at opendata.transport.nsw.gov.au and subscribe to the GTFS-Realtime, Trip Planner, and Live Traffic products). The `fuel ...` commands additionally need free FuelCheck credentials (`NSW_FUELCHECK_API_KEY` + `NSW_FUELCHECK_API_SECRET` from api.nsw.gov.au) — the CLI does the OAuth2 handshake and caches the token; without them the fuel commands fail with an actionable message and everything else still works.

Run `nsw-transport-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nsw-transport-pp-cli cameras --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
nsw-transport-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nsw-transport-pp-cli feedback --stdin < notes.txt
nsw-transport-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.nsw-transport-pp-cli/feedback.jsonl`. They are never POSTed unless `NSW_TRANSPORT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NSW_TRANSPORT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nsw-transport-pp-cli profile save briefing --json
nsw-transport-pp-cli --profile briefing cameras
nsw-transport-pp-cli profile list --json
nsw-transport-pp-cli profile show briefing
nsw-transport-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nsw-transport-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add nsw-transport-pp-mcp -- nsw-transport-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nsw-transport-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nsw-transport-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nsw-transport-pp-cli <command> --help`.
