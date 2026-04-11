# Weather CLI Absorb Manifest

## Sources Analyzed
1. **wttr.in** (Python/Go, curl service) — forecast display, multi-location, JSON, PNG, moon phases
2. **wthrr** (Rust CLI) — terminal weather companion
3. **open-meteo-mcp** (MCP) — 17 tools: forecast, archive, air quality, marine, flood, climate, elevation, geocoding, multi-model
4. **weather-mcp-server** (MCP) — NWS-based: alerts, forecast
5. **openmeteopy** (PyPI) — Open-Meteo Python wrapper
6. **NWS Python** (PyPI) — NWS API wrapper

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Current conditions | wttr.in, all CLIs | `weather now [location]` | Temp, feels-like, wind, humidity, UV, precip in one line. --json |
| 2 | Hourly forecast | wttr.in v2, MCP | `weather hourly [location]` | 24-48hr hourly breakdown. --hours flag |
| 3 | Daily forecast | wttr.in, all | `weather forecast [location]` | 7-14 day daily forecast. --days flag |
| 4 | Air quality | open-meteo-mcp | `weather air [location]` | AQI, PM2.5, PM10, ozone, pollen, UV. --json |
| 5 | Severe weather alerts | NWS MCP | `weather alerts [state-or-location]` | Active alerts by area. NWS data. --json |
| 6 | Historical weather | open-meteo-mcp | `weather history [location] --date YYYY-MM-DD` | Any date back to 1940. --json |
| 7 | Marine forecast | open-meteo-mcp | `weather marine [location]` | Wave height, period, sea temp |
| 8 | Geocoding | open-meteo-mcp | `weather locate <name>` | Resolve city/place to coordinates |
| 9 | Elevation | open-meteo-mcp | `weather elevation [location]` | Digital elevation for any point |
| 10 | Multi-model forecast | open-meteo-mcp | `weather models [location]` | Compare GFS, ECMWF, DWD, etc. side by side |
| 11 | Moon phases | wttr.in | `weather moon` | Current moon phase + upcoming |
| 12 | Sunrise/sunset | wttr.in v2 | Included in `now` and `forecast` output |
| 13 | Multiple locations | wttr.in | All commands accept location arg, default to saved location |
| 14 | JSON output | wttr.in format=j1 | `--json` on every command |
| 15 | Flood forecast | open-meteo-mcp | `weather flood [location]` | River discharge predictions |
| 16 | Climate projections | open-meteo-mcp | `weather climate [location]` | IPCC warming scenarios |

## Transcendence (user-first, self-vetted)

### Personas → Features

**Morning planner:** "Should I bring an umbrella? Is it safe to bike?"
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Morning brief | `weather` (no args) | 9/10 | Default command shows: current temp, feels-like, today's high/low, precip chance, wind, UV, and any active alerts for your saved location. One glance, done. | Uses Open-Meteo /forecast (hourly+daily) + NWS /alerts/active. No existing CLI gives forecast+alerts in one output. |
| 2 | Should I bike? | `weather bike` | 8/10 | Checks: temp (too hot/cold?), wind (>25mph warning), rain chance (>50% warning), feels-like, air quality. Returns GO/CAUTION/NO verdict with reasons. | Open-Meteo forecast + air quality endpoints. Mechanical: threshold checks on temp, wind, precip, AQI. |

**Parent / safety-conscious:**
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 3 | Alert monitor | `weather watch` | 8/10 | Poll NWS alerts every 5 minutes, print new alerts as they arrive. For severe weather tracking during storms. | NWS /alerts/active with polling. Like `tail` for weather alerts. |

**Traveler / event planner:**
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 4 | Compare locations | `weather compare <loc1> <loc2>` | 8/10 | Side-by-side forecast for two locations. Which is warmer? Which has less rain? Great for "should I go to the beach or the mountains?" | Two Open-Meteo /forecast calls, tabulate side by side. wttr.in has multi-location but not comparison view. |
| 5 | Event planner | `weather on <date> [location]` | 7/10 | "What's the weather Saturday?" Fetches specific date from the forecast range (if within 14 days) or historical average (if past or far future). | Open-Meteo /forecast for near dates, /archive for historical average of that day across past years. |

**Data nerd / climate curious:**
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 6 | Is this normal? | `weather normal [location]` | 8/10 | Compares today's conditions to the 30-year historical average for this date. "Today is 5°F above average for April 11." | Open-Meteo /forecast (today) vs /archive (same date, 1991-2020 average). Pure math comparison. |
| 7 | Year comparison | `weather year [location]` | 7/10 | "How does 2026 compare to 2025?" Monthly avg temp/rain for two years side by side. | Open-Meteo /archive for both years, aggregate by month. |

**Allergy sufferer / health:**
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 8 | Air quality brief | `weather breathe [location]` | 7/10 | Combined: AQI, pollen levels (grass/tree/ragweed), UV index, and recommendation (safe to exercise outdoors? keep windows open?). | Open-Meteo /air-quality endpoint has all these fields. Mechanical threshold checks. |

### Self-vet results
- All 8 features pass the 5 kill checks: no LLM, no external service beyond Open-Meteo+NWS, no auth required, all single-command scope, all testable.
- Buildability proof included in "How It Works" column.
- Dropped candidates: "weather mood" (map weather to activities — too subjective, fails LLM check), "weather clothing" (what to wear — subjective, fails verifiability check).
