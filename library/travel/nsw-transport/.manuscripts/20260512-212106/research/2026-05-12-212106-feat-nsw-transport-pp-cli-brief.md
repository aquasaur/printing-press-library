# NSW Transport (Sydney Location Intelligence) CLI Brief

A unified CLI over five NSW open-data APIs: TfNSW Public Transport GTFS-Realtime feeds,
the TfNSW Trip Planner API, Live Traffic NSW hazards, Live Traffic NSW cameras, and the
NSW FuelCheck fuel-price API. Slug: `nsw-transport`. Binary: `nsw-transport-pp-cli`.

## API Identity
- **Domain:** Sydney/NSW mobility — public transport realtime, journey planning, road hazards, traffic cameras, fuel prices.
- **Users:** Sydney commuters & developers, Home Assistant tinkerers, NSW drivers planning trips/fuel stops, agents answering "is my commute blocked / where's my bus / cheapest E10 near me" questions.
- **Data profile:** Realtime protobuf feeds (GTFS-RT TripUpdate/VehiclePositions/Alerts per mode), JSON journey plans (`rapidJSON`), GeoJSON hazard & camera FeatureCollections, JSON fuel-price station lists, GTFS static ZIPs.

## Reachability Risk
- **Low.** All hosts alive: `api.transport.nsw.gov.au` returns 401 without a key (auth-gated, expected); `api.onegov.nsw.gov.au` (FuelCheck) returns 401; `opendata.transport.nsw.gov.au` returns 200. No bot protection. Known operational gotchas (not blockers): HTTP 503 "quota/rate limit exceeded" when polling faster than every 15s; HTTP 500 "Policy Falsified" on malformed requests or unsubscribed API products; Aug-2025 Trip Planner engine upgrade broke `stop_finder` with `type_sf=stop` — use `type_sf=any`; the `version` query param is silently ignored; FuelCheck unauthenticated rate limit is 5 calls/min.

## Source Priority (combo CLI)
Confirmed ordering (from `source-priority.json`, user-confirmed):
1. **TfNSW Realtime (GTFS-Realtime)** — PRIMARY. Headline commands, top of README. Spec state: **no JSON spec — protobuf feeds, hand-built** using `github.com/MobilityData/gtfs-realtime-bindings`. Auth: free (`Authorization: apikey` header).
2. **TfNSW Trip Planner** — Spec state: docs-derived in-repo JSON spec (5 REST endpoints, `rapidJSON`). Auth: free (same apikey header).
3. **Live Traffic Hazards** — Spec state: docs-derived JSON spec (GeoJSON, `{category}/{type}` path). Auth: free (same apikey header).
4. **Live Traffic Cameras** — Spec state: docs-derived JSON spec (1 GeoJSON endpoint; images are JPGs on livetraffic.com). Auth: free (same apikey header).
5. **FuelCheck** — Spec state: docs-derived, hand-built client (OAuth2 client_credentials + `apikey`/`transactionid`/`requesttimestamp` headers, different host `api.onegov.nsw.gov.au`). Auth: free but separate credentials (`NSW_FUELCHECK_API_KEY` + `NSW_FUELCHECK_API_SECRET`).
- **Economics:** Everything is free. The split is auth-mechanism, not paid/free. Primary commands (`realtime …`) need only `NSW_OPENDATA_API_KEY`. `fuel …` commands need the two FuelCheck creds; without them, `fuel` commands fail with an actionable error and the rest of the CLI works.
- **Inversion risk:** Trip Planner has a richer documented surface than the protobuf Realtime feeds, and is spec-driven (more endpoints land in the scaffold). Do NOT let scaffold completeness invert the headline — `realtime` commands lead the README and the first-run flow per the user's stated order. Realtime commands are hand-built but are the headline.

## Auth Architecture
- **Primary (`api.transport.nsw.gov.au`):** `type: api_key`, `in: header`, `header: Authorization`, `prefix: apikey` → emits `Authorization: apikey <KEY>`. Env var `NSW_OPENDATA_API_KEY`. Register: https://opendata.transport.nsw.gov.au/data/user/register → create an application → subscribe to the relevant API products (GTFS-RT, Trip Planner, Live Traffic). Covers sources 1–4.
- **FuelCheck (`api.onegov.nsw.gov.au`):** hand-built `internal/source/fuelcheck/`. OAuth2: `GET /oauth/client_credential/accesstoken?grant_type=client_credentials` with `Authorization: Basic base64(key:secret)` → `{access_token, expires_in≈43199}` (~12h TTL, cached to config dir). Subsequent calls send `Authorization: Bearer <token>`, `apikey: <key>`, `transactionid: <uuid v4>`, `requesttimestamp: DD/MM/YYYY HH:MM:SS AM/PM`, `Content-Type: application/json`. Env vars `NSW_FUELCHECK_API_KEY`, `NSW_FUELCHECK_API_SECRET`. Register: https://api.nsw.gov.au/Product/Index/22.
- `doctor` verify path for the primary host: `/v1/carpark/full-list` (no params, returns 2xx with a valid key).

## API Surface (ground truth from research)

### Source 1 — GTFS-Realtime (hand-built)
- `GET https://api.transport.nsw.gov.au/{v}/gtfs/realtime/{mode}` — TripUpdate protobuf. v2 current for `sydneytrains`, `metro`, `lightrail` (inner west); v1 current for `buses`, `ferries`, `nswtrains`, `regionbuses`.
- `GET https://api.transport.nsw.gov.au/{v}/gtfs/vehiclepos/{mode}` — VehiclePosition protobuf. v2 for `sydneytrains`, `metro`, `lightrail`; v1 for `buses`, `ferries`, `regionbuses`.
- `GET https://api.transport.nsw.gov.au/{v}/gtfs/alerts/{mode}` — ServiceAlert protobuf. v2 covers `all`, `sydneytrains`, `metro`, `lightrail`, `buses`, `ferries`, `nswtrains`, `regionbuses`.
- `GET https://api.transport.nsw.gov.au/v1/gtfs/schedule/{mode}` — GTFS static ZIP (`/sydneytrains`, `/buses`, `/buses/{operator}`, `/ferries/{operator}`, `/lightrail/{operator}`, `/nswtrains`, `/regionbuses/{operator}`, `/metro`).
- Modes that take an operator suffix: `buses`, `ferries`, `lightrail`, `regionbuses` (e.g. `buses/SBSC008`); bare mode bundles all operators where supported.
- Response content-type: `application/x-google-protobuf`. Standard `gtfs-realtime.proto` covers base fields; TfNSW extension `1007` adds occupancy/formation (optional).

### Source 2 — Trip Planner (spec-driven), base `https://api.transport.nsw.gov.au/v1/tp`, all GET, `outputFormat=rapidJSON`, `coordOutputFormat=EPSG:4326`
- `GET /stop_finder` — find stops/POIs by name. Params: `name_sf` (query), `type_sf=any` (default; **not `stop`** per Aug-2025 break), `TfNSWSF=true`. Envelope: `{version, locations:[{id,name,type,coord,modes}]}`.
- `GET /trip` — plan A→B. Params: `name_origin`, `type_origin=any|stop|coord`, `name_destination`, `type_destination`, `depArrMacro=dep|arr`, `itdDate=YYYYMMDD`, `itdTime=HHMM`, `TfNSWTR=true`, `calcNumberOfTrips=5` (max 10), `wheelchair`, `excludedMeans=checkbox`+`exclMOT_{n}=1` (1=train,4=bus,5=ferry,7=coach,9=light rail,11=school bus...). Envelope: `{version, journeys:[{legs:[{duration, transportation:{product:{class},destination}, origin:{name,departureTimePlanned}, destination:{name,arrivalTimePlanned}, stopSequence, infos}]}]}`. (No fares since Oct 2023.)
- `GET /departure_mon` — departure board for a stop. Params: `name_dm=<stopId>`, `type_dm=stop`, `mode=direct`, `departureMonitorMacro=true`, `TfNSWDM=true`, `itdDate`, `itdTime`. Envelope: `{version, stopEvents:[{departureTimePlanned, departureTimeEstimated, isRealtimeControlled, transportation:{number,destination}, location:{id,name}}]}`.
- `GET /coord` — stops near a coordinate. Params: `coord=<lng:lat:EPSG:4326>`, `inclFilter=1`, `type_1=GIS_POINT`, `radius_1=<m>`. Envelope: `{version, locations:[{id,name,type,coord,distance}]}`.
- `GET /add_info` — service-status messages. Params: `filterPublicationStatus=current`, `filterDateValid=YYYYMMDD`, `filterMOTType=<n>`. Envelope: `{version, infos:{current:[{id,type,priority,title,content,affectedLines,validityPeriods}]}}`.

### Source 3 — Live Traffic Hazards (spec-driven), base `https://api.transport.nsw.gov.au/v1/live/hazards`
- `GET /{category}/{type}` — GeoJSON FeatureCollection. `{category}` ∈ `incident|fire|flood|alpine|roadwork|majorevent|regional-lga-incident` (plus `regional-lga-participation` which only supports `/all`). `{type}` ∈ `all|open|closed`. Feature properties: `id`, `headline`, `displayName`, `mainCategory`, `subCategory`, `roads:[{region,roadName,conditionTendency,extent,impactedLanes}]`, `created`, `lastUpdated`, `start`, `end`, `isMajor`, `isEnded`, `otherAdvice`, `webUrl`, `arrangementElements`, `periods`, `attendingGroups`, `publicTransport`. Geometry: `Point` `[lng,lat]` WGS84.

### Source 4 — Live Traffic Cameras (spec-driven), base `https://api.transport.nsw.gov.au/v1/live/cameras`
- `GET /` — GeoJSON FeatureCollection, one Feature per camera. Properties: `id`, `region`, `title`, `view`, `direction`, `href` (direct JPG URL on livetraffic.com, rotates ~every 15s). No per-camera or per-region sub-path — filter client-side; fetch the image by GETting `href`.

### Source 5 — FuelCheck (hand-built), base `https://api.onegov.nsw.gov.au/FuelPriceCheck/v1/fuel` (v2 also exists: NSW+TAS)
- `GET /prices` — all current prices. `{stations:[{stationcode,brand,name,address,location:{latitude,longitude}}], prices:[{stationcode,fueltype,price,lastupdated}]}`.
- `GET /prices/new` — prices updated since this apikey's last call today (server-side cursor, resets midnight AEST; driven by the `requesttimestamp` header).
- `POST /prices/nearby` — body `{fueltype, latitude, longitude, radius (km), brand:[...], sortby, sortascending}` → stations+prices in radius sorted.
- `GET /prices/station/{stationcode}` — `{station:{...}, prices:[{fueltype,price,lastupdated}]}`.
- `POST /prices/location` — body `{fueltype, namedlocation, stateTerritory, suburb, postcode, brands:[...]}` → prices by named location.
- `GET /lovs` — reference lists: `{fueltypes:[{code,name}], brands:[{brandid,name}], sortfields:[...]}`. Fuel codes: `E10,U91,E85,P95,P98,DL,PDL,B20,LPG,CNG,EV`.
- `GET /prices/trends` — params `fueltype` (required), `period` ∈ `DAY|WEEK|MONTH|YEAR` → statewide average trend. (Path listed in portal; verify at runtime.)
- `GET /prices/station/{stationcode}/trends` — same params, per-station. (Verify at runtime.)
- `GET /v2/fuel/prices/widget` — aggregate summary by fuel type. (Verify at runtime.)

### Bonus — Car Park Availability (spec-driven, optional), base `https://api.transport.nsw.gov.au/v1/carpark`
- `GET /full-list` — consolidated list of all Park & Ride facilities + availability.
- `GET /` with `?facility=<id>` — availability for one facility.

## Top Workflows
1. **"Where's my bus / when does it leave?"** — realtime vehicle positions + departure monitor for a stop.
2. **"Plan my journey A→B"** — Trip Planner `trip`, with leg-by-leg breakdown, realtime delays, and accessibility filtering.
3. **"Is my commute blocked right now?"** — Live Traffic hazards (incidents/roadworks/floods/fires) on the roads near my route + Trip Planner alerts for affected lines.
4. **"Cheapest fuel near me / near a place I'm driving to"** — FuelCheck nearby/location, sorted by price, filtered by fuel type & brand.
5. **"What's disrupted on the network today?"** — GTFS-RT alerts + Trip Planner `add_info` aggregated across all modes.
6. **"Show me the traffic camera at <location>"** — find camera by region/road, fetch the JPG.

## Table Stakes (from competing tools — Home Assistant integrations, Raycast extension, PyPI libs)
- Next departure(s) from a stop, with mode/route/destination filters, count N — HA `transport_nsw`, `ha_transportnsw`, Raycast.
- Stop search / validate stop IDs — `pytransportnswv2`, Raycast.
- Full trip plan A→B with leg-by-leg detail (platform, times, changes, every stop) — `pytransportnswv2`, `ha_transportnsw`, Raycast.
- Service alerts as queryable data — `ha_transportnsw` alerts sensor.
- GTFS-RT vehicle positions / trip updates / alerts ingestion — `TfNSW_GTFSRDB` (DB pipeline only — no live query CLI).
- Fuel prices by station, by fuel type — HA `nsw_fuel_station`.
- Fuel prices by radius from lat/lon, by named location, by brand — `nsw-fuel-api-client` (library only — no CLI, no "cheapest near me").
- Live Traffic hazards — **nothing exists** (complete gap).
- Live Traffic cameras — **nothing exists** (complete gap).
- MCP server for any of these — **nothing exists** (complete gap).

## Data Layer (local SQLite store)
- **Primary entities to sync & FTS-index:**
  - `stops` — from Trip Planner `stop_finder`/`coord` over time + (optionally) parsed GTFS static `stops.txt`. Key gravity entity: id, name, type, lat/lng, modes. FTS on name.
  - `cameras` — full GeoJSON FeatureCollection from `/v1/live/cameras` (≈hundreds). id, region, title, view, direction, href, lat/lng. FTS on title+region+view.
  - `fuel_stations` — from FuelCheck `/prices` station list (≈2k NSW stations). stationcode, brand, name, address, suburb, lat/lng. FTS on name+address.
  - `fuel_prices` — latest price per (stationcode, fueltype) with `lastupdated`. Enables offline "cheapest near me" and price-history snapshots.
  - `hazards` — snapshots of open hazards across categories (id, headline, mainCategory, roads, lat/lng, created, lastUpdated, isMajor, isEnded). Enables "what changed since I last looked" and "hazards near a route".
  - `alerts` — GTFS-RT alerts + Trip Planner `add_info` normalized (id, mode, severity, header, description, affected_lines, active_period). FTS on header+description.
  - `carparks` — Park & Ride facilities + last availability snapshot.
- **Sync cursors:** FuelCheck `/prices/new` (server cursor) for incremental fuel; per-category last-fetch timestamps for hazards; full re-fetch for cameras/stations (small, static-ish).
- **FTS/search:** unified `nsw-transport-pp-cli search "<term>"` across stops, cameras, fuel stations, hazards, alerts.
- **Transcendence enabled by the store:** "cheapest fuel near stop X" (join stops + fuel_stations by distance), "fuel price drift since yesterday" (fuel_prices snapshots), "hazards on my planned route" (join trip legs' roads ↔ hazards by road name / proximity), "what's newly disrupted" (alerts diff).

## Product Thesis
- **Name:** "NSW Transport" / "Sydney Mobility" — display name "Transport for NSW Open Data".
- **Why it should exist:** Today these five APIs are scattered across Home Assistant sensors, a Raycast extension (trains only), two stale PyPI libraries, and *nothing at all* for Live Traffic hazards/cameras or "cheapest fuel near me." There is no CLI, no agent surface, and nothing that joins transit + roads + fuel. This CLI gives you (a) every existing feature with `--json`/`--select`/typed exit codes, (b) a local SQLite store so "cheapest E10 near Town Hall" is one offline query, (c) cross-source commands no single API offers ("is my commute blocked", "fuel stop on my drive", "where's my bus on route 333"), and (d) an MCP surface so an agent can answer Sydney mobility questions directly. First tool to treat Sydney location intelligence as one thing.

## Build Priorities
1. **Priority 0 — data layer:** store schema for stops, cameras, fuel_stations, fuel_prices, hazards, alerts, carparks; `sync` (full + incremental), `search` (FTS), `sql`.
2. **Priority 1 — absorb (match everything):** all Trip Planner endpoints (`trip`, `stop_finder`/`stops`, `departure_mon`/`departures`, `coord`/`nearby`, `add_info`/`alerts`); GTFS-RT `realtime vehicles/trips/alerts <mode>` + `realtime schedule <mode>` (ZIP download); Live Traffic `traffic hazards <category> [open|closed|all]`; `traffic cameras` (list/filter) + `traffic camera <id> --save <file>`; FuelCheck `fuel prices`, `fuel near`, `fuel station <code>`, `fuel by-location`, `fuel types`, `fuel trends`; `carpark list` / `carpark show`.
3. **Priority 2 — transcend:** cross-source commands listed under Data Layer + `commute` (named-route check: trip plan + hazards near the legs' roads + line alerts in one go), `fuel-stop` (cheapest fuel of a given type within N km of origin/destination/along a planned trip), `whereis` (realtime vehicles on a named route/line, nearest to a stop), `disruptions` (today's network disruptions across all modes from alerts + add_info), `cameras-near` (cameras within N km of a lat/lng or stop), `fuel-drift` (price change since last snapshot, per fuel type or brand).
4. **Priority 3 — polish:** enrich flag descriptions (mode lists, fuel codes, hazard categories), tests for the GTFS-RT decoder / GeoJSON parser / distance math / FuelCheck OAuth, README cookbook, MCP read-only annotations on all read commands.
