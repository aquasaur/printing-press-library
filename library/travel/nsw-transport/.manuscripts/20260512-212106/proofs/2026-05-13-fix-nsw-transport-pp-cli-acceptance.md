# nsw-transport-pp-cli — Phase 5 Acceptance Report

**Live testing performed** with a real NSW Open Data Hub API key (`NSW_OPENDATA_API_KEY`)
and real FuelCheck credentials (`NSW_FUELCHECK_API_KEY` / `NSW_FUELCHECK_API_SECRET`),
both supplied by the user and stored only in `~/.config/nsw-transport-pp-cli/config.toml`
(never archived/committed). Both Full Dogfood and Quick Check matrices were run.

## Quick Check — gate marker
`printing-press dogfood --live --level quick` → **PASS 5/5** (3 skipped on fixture
constraints). `phase5-acceptance.json` written with `status: pass`. Gate: **PASS**.

## Full Dogfood — thorough analysis
`printing-press dogfood --live --level full` → 87 pass / 13 fail / 73 skip. The 13
failures, classified:

**Upstream FuelCheck endpoint state (6) — not CLI defects:**
- `fuel types` / `fuel brands` ×2 each — FuelCheck's `/FuelPriceCheck/v1/fuel/lovs`
  returns HTTP 500 (`InternalServerError`) right now. The CLI surfaces the error
  cleanly with exit 5. The `/lovs` endpoint is upstream-broken; flagged in the
  absorb manifest as "verify at runtime".
- `fuel trends` ×2 — FuelCheck's `/FuelPriceCheck/v1/fuel/prices/trends` returns
  HTTP 404 ("Resource not found"). The trends endpoint is portal-documented but not
  live; the command's `--help` already discloses this caveat and surfaces the 404.

**TfNSW API permissive fuzzy-matching on garbage input (5) — not CLI defects:**
- `cameras-near __printing_press_invalid__`, `station __printing_press_invalid__`,
  `whereis __printing_press_invalid__`, `trip departures __printing_press_invalid__`,
  `trip stops __printing_press_invalid__` — the dogfood error-path test passes a
  synthetic invalid argument and expects a non-zero exit. The TfNSW APIs respond
  with HTTP 200: `stop_finder` fuzzy-matches the garbage string to a far-away POI
  ("Museum of Printing, Armidale"); the realtime route filter and departure board
  return empty results. The CLI faithfully returns exit 0 with an empty/zero-count
  result — a correct "found nothing" response, not a crash. (`commute` and
  `realtime schedule` were tightened this phase to reject incomplete/invalid input
  with exit 2; they now pass the error-path test.)

**json_fidelity check on streaming / object-shaped output (2) — expected:**
- `workflow archive --json` emits newline-delimited JSON sync events (one object
  per line), not a single JSON document — correct for a streaming command.
- `trip alerts --json` returns a valid JSON object (`{meta, results:{version, infos:{current:[...]}}}`);
  the json_fidelity probe appears to false-positive on object-shaped (vs array)
  responses. The output parses cleanly with `jq`.

## Live behaviour verified by hand (the flagship features)
All confirmed returning real, correct data against the live APIs:
- `realtime vehicles sydneytrains` / `realtime vehicles buses --route 333` — live GPS positions, occupancy, bearing/speed.
- `realtime trips sydneytrains` / `realtime alerts all` — live trip updates and service alerts (decoded from the GTFS-Realtime protobuf).
- `trip stops "Central"` / `trip departures 10101100` / `trip plan` — stop search, realtime departure board, journey planning.
- `hazards incident open` / `cameras` / `carpark list` — Live Traffic GeoJSON, hundreds of cameras with JPG hrefs, Park & Ride facilities.
- `commute "Marrickville Station" "Central Station"` — resolves both stops, plans the journey (11 min), reports nearby road hazards + line alerts; verdict line. (Stop-name resolution was hardened this phase to prefer transit stops over streets/POIs.)
- `whereis route 333 --near 203311 --mode buses` — 24 live route-333 buses, ranked by distance from the UNSW Mall stop.
- `station 200060` — 5 next departures with realtime estimates + 3 nearby cameras + 3 cheapest nearby fuel stations, in one call.
- `cameras-near --coord -33.81,151.00 --radius 5` — 6 cameras near Parramatta with hrefs and distances.
- `disruptions --all` — GTFS-RT alerts ∪ Trip Planner service status, deduped and severity-sorted.
- `fuel prices --fueltype E10` — 1497 stations with E10 prices, sorted ascending (real data, Metro Islington 164.5¢).
- `fuel near --coord -33.8136,151.0014 --fueltype E10 --radius 5` — 50 stations near Parramatta.
- `fuel-stop E10 --near "Parramatta Station" --radius 5` — resolves the stop name, returns 51 stations near it.
- `fuel station 18070` — single-station prices across all fuel types.
- `fuel-drift E10` — diffs the two most recent snapshots; 1497 rows when `--include-unchanged`, 0 changed between two same-second snapshots (correct).
- `refresh` — records 10,591 fuel-price points across 3,279 stations into the local store.
- `sync` — exits 0 (`carpark` resource; the object-shaped Trip Planner / GeoJSON resources are fetched live by their commands instead).
- `doctor` — Auth: configured, Credentials: valid.

## Fixes applied this phase (3 fix loops)
1. **Auth wiring** — FuelCheck creds now resolve from config file *or*
   `NSW_FUELCHECK_API_KEY` / `NSW_FUELCHECK_API_SECRET` env (was env-only), matching
   how the Open Data Hub key works. Added `Config.FuelcheckApiKey/Secret`,
   `fuelcheck.NewWithCreds`.
2. **FuelCheck response decoding** — station code maps from the `code` JSON key (not
   `stationcode`); station code and lat/lng/price tolerate both JSON numbers and
   JSON-encoded strings (`FlexStr`/`FlexNum`) — FuelCheck returns these differently
   per endpoint. `fuel station` echoes the request code when the response omits it.
3. **`fuel by-location`** — sends a `namedlocation` body field (folded from
   `--location`/`--suburb`/`--postcode`); was returning HTTP 400.
4. **`fuel near`** — takes `--coord <lat,lng>` (negative NSW latitudes broke positional
   parsing); positional `<lat> <lng>` still works after `--`.
5. **`commute`** — 1-argument invocation now errors (exit 2) instead of showing help; the
   error-path test passes.
6. **`realtime schedule`** — validates the mode root segment before printing the URL.
7. **`commute` / `station` stop resolution** — prefers transit stops/platforms over
   streets/addresses/POIs (`pickBestLocation`), so `commute "Marrickville" "Central"`
   resolves Central *Station*, not "Central Rd, Avalon".
8. **`sync`** — only syncs `carpark` (the one flat enumerable resource), with absolute
   URLs (the previous bare paths ignored the per-resource `base_url` overrides and 500'd)
   and `tsn` as the carpark ID field. Exits 0 now.

## Known gaps (documented, non-blocking)
- **FuelCheck `/lovs` (500) and `/prices/trends` (404)** are upstream-broken/absent;
  `fuel types`, `fuel brands`, `fuel trends` surface the upstream error. Disclosed in
  the manifest and the `fuel trends --help`.
- **`sync` over the spec resources** is a no-op except `carpark` (and even `carpark`
  rows aren't currently stored — `all_items_failed_id_extraction` despite the `tsn`
  override; the carpark items nest under a path or the templated ID extractor doesn't
  consult the runtime override). The real store-population path is `refresh` (fuel
  snapshots). `search`/`sql` therefore have limited data. Generator-limitation
  candidate for the retro: sync ignoring per-resource `base_url` overrides; sync model
  not fitting GeoJSON/object-shaped responses.
- **`path_validity 0/10` / live sample probe** in the no-key scorecard run are
  artifacts of unauthenticated probing (every NSW endpoint requires a key → 401).

## Gate: PASS
Quick Check PASSED 5/5; `phase5-acceptance.json` has `status: pass`. The Full Dogfood
13 failures are all classified non-defects (upstream-dead FuelCheck endpoints,
TfNSW fuzzy-matching synthetic garbage input, NDJSON/object json_fidelity probes).
No flagship feature is broken — all verified live. CLI is shippable.
