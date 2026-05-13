# NSW Transport CLI — Absorb Manifest

Combo CLI over 5 NSW open-data APIs. Slug `nsw-transport`, binary `nsw-transport-pp-cli`.
Primary source (per user-confirmed order): **TfNSW Realtime (GTFS-Realtime)**.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Next departure(s) from a stop, mode/route/destination filters, count N | HA `transport_nsw` / `ha_transportnsw` / Raycast `transport-nsw` | `trip departures <stopId> --count N --mode --route --to` (Trip Planner `departure_mon`) | `--json`/`--select`/`--csv`, typed exit codes, `--to` destination filter, scriptable; no app required |
| 2 | Stop/station/wharf/POI search | `pytransportnswv2` / Raycast | `trip stops "<query>"` (`stop_finder`, `type_sf=any`) | offline FTS once synced, returns id+coord+modes, agent-native |
| 3 | Validate stop IDs / stop metadata | `pytransportnswv2` | covered by `trip stops` + local `stops` table | offline, composable with `whereis`/`fuel-stop` |
| 4 | Full trip plan A→B, leg-by-leg (platform, times, changes, every stop) | `pytransportnswv2` / `ha_transportnsw` / Raycast | `trip plan <origin> <destination> --depart/--arrive --date --time --wheelchair --exclude bus,ferry --trips N` | leg-by-leg `--json`, accessibility & mode-exclusion filters, realtime delays inline |
| 5 | Stops near a coordinate | `transport-nsw-opendata` (rycus86) | `trip nearby <lat> <lng> --radius` (`coord`) | sorted by distance, `--json`, feeds the store |
| 6 | Service-status / disruption messages (Trip Planner) | `ha_transportnsw` alerts sensor | `trip alerts --mode --current` (`add_info`) | queryable data not a single sensor; feeds local `alerts` |
| 7 | GTFS-RT vehicle positions (where are vehicles now) | `TfNSW_GTFSRDB` (DB ingest only) | `realtime vehicles <mode> [operator] --version v1\|v2` (protobuf decode) | live query, not a DB pipeline; `--json`, route/trip filter, occupancy where present |
| 8 | GTFS-RT trip updates (delays/cancellations per trip) | `TfNSW_GTFSRDB` | `realtime trips <mode> --version v1\|v2` (protobuf decode) | live query, delay summary, `--json` |
| 9 | GTFS-RT service alerts feed | `TfNSW_GTFSRDB` / `ha_transportnsw` | `realtime alerts <mode>` (protobuf decode) | live query, feeds local `alerts` (unified with `add_info`) |
| 10 | GTFS static timetable download | (manual) | `realtime schedule <mode> [operator] --output <dir>` (ZIP) | named modes/operators, writes the ZIP, prints contents summary |
| 11 | Fuel prices for a station | HA `nsw_fuel_station` | `fuel station <stationcode>` | `--json`/`--select`, all fuel types at once, feeds store |
| 12 | All current fuel prices | `nsw-fuel-api-client` (library) | `fuel prices --fueltype --brand` | CLI surface, brand/type filter, `--csv` for spreadsheets |
| 13 | Fuel prices near a lat/lng within radius, by type & brand, sorted | `nsw-fuel-api-client` (library, no CLI, no sort) | `fuel near <lat> <lng> --fueltype --radius --brand --sort price` (`prices/nearby`) | sort by price/distance, `--json`, the basis for `fuel-stop` |
| 14 | Fuel prices by named location (suburb/postcode) | `nsw-fuel-api-client` | `fuel by-location --suburb/--postcode --fueltype --brand --sort price` (`prices/location`) | CLI surface, sort, `--json` |
| 15 | Fuel reference lists (fuel codes, brands) | `nsw-fuel-api-client` | `fuel types` / `fuel brands` (`lovs`) | offline once synced, used to validate `--fueltype` |
| 16 | Statewide / per-station fuel price trends | (none — gap) | `fuel trends --fueltype --period DAY\|WEEK\|MONTH\|YEAR [--station <code>]` | first CLI for it; `--json` time-series |
| 17 | Live Traffic hazards (incidents, roadworks, floods, fires, alpine, major events) | (none — complete gap) | `traffic hazards <category> [open\|closed\|all] --major --road <name>` | first CLI/agent surface; category+status+road filter, `--json` GeoJSON-flattened |
| 18 | Live Traffic cameras list | (none — complete gap) | `traffic cameras --region --road --view` | first CLI; region/road/view filter, prints JPG hrefs, feeds store |
| 19 | Fetch a specific traffic camera image | (none — complete gap) | `traffic camera <id> --save <file>` (default: print href; `--save` downloads the JPG) | print-by-default side-effect command, `--save` opt-in |
| 20 | Park & Ride car-park availability | (manual) | `carpark list` / `carpark show <facilityId>` | consolidated `/full-list`, `--json`, feeds store |
| 21 | Unified offline FTS search across stops/cameras/fuel stations/hazards/alerts | (none) | `search "<term>"` | one query, ranked, `--json` |
| 22 | Local SQLite sync of all entities | (none) | `sync` (full + incremental via FuelCheck `prices/new`) | makes every transcendence command offline-capable |

**Stubs:** none. Two FuelCheck endpoints (`/prices/trends`, `/v2/fuel/prices/widget`) are documented in the portal but not cross-validated by community implementations — `fuel trends` ships fully wired against `/v1/fuel/prices/trends` and surfaces an honest error if the endpoint shape differs at runtime; it is **not** a stub.

## Transcendence (only possible with our approach)

From the Phase 1.5c.5 brainstorm subagent (customer model → 2× candidates → adversarial cut; 7 survivors ≥ 6/10):

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Commute-blocked check | `commute "Marrickville" "Central" --depart 0815` | 8/10 | Joins Trip Planner `trip` legs ↔ local `hazards` (road-name match) ↔ local `alerts` (lines used) into one verdict — no API or existing tool fuses transit + roads + line alerts. |
| 2 | Cheapest fuel near a place / destination | `fuel-stop E10 --near "Central" --radius 5 --sort price` | 8/10 | Resolves a named stop/POI (or trip destination) to coords via local `stops`, then ranks local `fuel_prices` by distance — works offline; `nsw-fuel-api-client` is a library with no "cheapest near me" and no place-name input. |
| 3 | Where's my bus on a route | `whereis route 333 --near 203311` | 8/10 | Decodes GTFS-RT `vehiclepos/{mode}`, filters to one route_id, ranks vehicles by distance to a chosen stop from local `stops` — `TfNSW_GTFSRDB` is a DB pipeline with no live query. |
| 4 | Place snapshot (gravity-entity join) | `station 2204135` | 7/10 | Joins local `stops` ↔ `alerts`/`cameras`/`fuel_prices`/`carparks` plus a live `departure_mon` call — the canonical "tell me everything about this stop" agent query; no single API returns it. Descoped to a fixed set (next 3 departures, line alerts, nearest 2 cameras, cheapest E10 within 3 km, nearest Park & Ride). |
| 5 | Cameras near a location | `cameras-near 203311 --radius 3` (or `--coord -33.8688,151.2093`) | 6/10 | Local `cameras` table + distance math against a stop or lat/lng — the live endpoint is one un-paged GeoJSON blob with no proximity query. |
| 6 | Network disruptions, all modes, deduped | `disruptions --all` (or `--mode sydneytrains`) | 6/10 | Reads local `alerts` = GTFS-RT service alerts ∪ Trip Planner `add_info`, normalized + deduped + severity-sorted — neither source endpoint gives the union. |
| 7 | Fuel price drift since last sync | `fuel-drift E10 --brand "7-Eleven"` | 6/10 | Diffs `fuel_prices` snapshots in local SQLite between syncs — there is no API endpoint for "what changed since I last looked". |

**Killed in the cut (audit trail):** `next --to` (folded into absorbed #1), `route-board` (scope creep → split into `whereis` + absorbed `trip departures` + `disruptions`), `carpark-near` (folded into `station`), `hazards-near` (overlaps `commute` + `cameras-near`/`station`; covered by absorbed `traffic hazards --road`), `route-hazards` (redundant with `commute`), `alpine-status` (niche/seasonal/thin rename), `fuel-cheapest` (thin `--sort price | head -1`).

## Source priority & inversion note

Per `source-priority.json` the order is Realtime → Trip Planner → Hazards → Cameras → FuelCheck. Realtime ends up with **fewer total commands** (4 absorbed + `whereis` = 5) than Trip Planner (5 absorbed + parts of `commute`/`disruptions`/`station`) and FuelCheck (7 absorbed + `fuel-stop`/`fuel-drift`), because Realtime is a small protobuf surface and the others have richer JSON surfaces. This is **not** a discovery-path failure — Realtime is fully hand-built and is the headline. README leads with `realtime` and `whereis`; first-run flow starts there. User confirmed this order explicitly at the priority gate.
