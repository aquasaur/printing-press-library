# nsw-transport-pp-cli — Build Log

## Generated (Printing Press)
- Spec-driven scaffold from `nsw-transport-spec.yaml`: `trip` resource (stops, plan, departures, nearby, alerts), `hazards` (promoted top-level), `cameras` (promoted), `carpark` (list, show). Plus the standard framework: doctor, auth, sync, search, export/import, workflow, api, agent-context, which, profile, feedback.
- Auth: `api_key` with `format: "apikey {token}"` → `Authorization: apikey <KEY>`, env `NSW_OPENDATA_API_KEY`. All 8 quality gates passed at generate time.

## Hand-built (Phase 3)
- `internal/geo/` — Haversine distance helper (+ tests).
- `internal/source/realtime/` — GTFS-Realtime protobuf client via `github.com/MobilityData/gtfs-realtime-bindings`: vehicle positions, trip updates, service alerts (v1/v2 per mode), GTFS static ZIP download. Adaptive rate limiter, surfaces `*cliutil.RateLimitError`. (+ tests for mode validation, URL building, default version.)
- `internal/source/fuelcheck/` — FuelCheck client: OAuth2 client-credentials handshake with on-disk token cache, then `apikey`/`transactionid`/`requesttimestamp` headers; endpoints `prices`, `prices/new`, `prices/nearby` (POST), `prices/station/{code}`, `prices/location` (POST), `lovs`, `prices/trends` (+ per-station). Reads `NSW_FUELCHECK_API_KEY` / `NSW_FUELCHECK_API_SECRET`; `MissingCredsError` → config exit 10. Adaptive limiter. (+ tests for uuid/transactionid, parseSeconds.)
- `internal/nsw/` — local SQLite store for the location-intelligence commands: `nsw_stations` + `nsw_fuel_snapshots` tables, `SnapshotFuel`, `FuelDrift` (compares the two most recent snapshots), `ErrNoHistory`. (+ end-to-end FuelDrift test with a temp db.)
- Commands wired in `root.go`:
  - **Absorbed (Priority 1):** `realtime vehicles|trips|alerts|schedule`, `fuel prices|near|station|by-location|types|brands|trends`. (Trip Planner / hazards / cameras / carpark are the generated scaffold.)
  - **Transcendence (Priority 2):** `commute`, `fuel-stop`, `whereis`, `station`, `cameras-near`, `disruptions`, `fuel-drift`, plus `refresh` (records a fuel-price snapshot for `fuel-drift`).
- All hand-written commands: verify-friendly RunE (`len(args)==0 → Help()`, `dryRunOK` short-circuit, no `cobra.MinimumNArgs`/`MarkFlagRequired`), `--json`/`--select`/`--csv` via `printJSONFiltered`, `mcp:read-only` annotations on read commands, `pp:typed-exit-codes` declared, side-effect commands (`realtime schedule --output`, `cameras-near --save`) print-by-default + `cliutil.IsVerifyEnv()` short-circuit, `// pp:client-call` on hidden-helper call sites.

## Intentionally deferred / out of scope
- GTFS-RT TfNSW extension fields (`1007` occupancy/formation) — base GTFS-RT only.
- A full offline cache of stops/cameras/hazards in the local store — `search` (generated) and `sync` (generated) operate on the spec resources only; the cross-source commands fetch live (slower but correct). Only `fuel-drift` requires persistence and gets a dedicated `refresh`/snapshot path.
- `carpark` proximity matching in `station` (no clean facility↔stop join) — `station` covers departures, service status, nearby cameras, nearby fuel.
- `fuel trends` / `fuel/prices/widget` are wired but the endpoint shape is portal-documented, not community-validated; `fuel trends` prints raw JSON if the shape differs.

## Generator limitations found
- The internal spec `response_format` supports only `json`/`html` — protobuf GTFS-RT feeds and GTFS ZIP downloads must be hand-built. (Same shape as the GraphQL-only carve-out.)
- One `auth:` block per spec — a combo CLI with a second OAuth-on-a-different-host source (FuelCheck) requires a hand-built sibling client. (Retro candidate: a `secondary_auth` / per-resource auth-override concept, or document the sibling-client pattern more prominently.)
- Param `aliases` must be lowercase kebab-case — can't use the upstream wire name (`name_sf`) as an alias; dropped them (the `flag_name` public name is what matters anyway).
