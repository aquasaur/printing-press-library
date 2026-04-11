# Weather CLI Brief

## API Identity
- Domain: Weather forecasts, historical climate data, air quality, severe weather alerts
- Users: Everyone who goes outside — developers, runners, cyclists, gardeners, parents, travelers, people planning outdoor events
- Data profile: Dual-API — Open-Meteo (global forecasts, 80yr history, air quality, marine, climate) + NWS (US severe weather alerts, observation stations)

## Data Sources

### Primary: Open-Meteo (global, no auth, unlimited)
- Base URL: https://api.open-meteo.com/v1
- Endpoints:
  - /forecast — 14-day forecast (hourly + daily), temp, wind, precip, humidity, UV, sunrise/sunset
  - /archive — historical hourly data from 1940 onward
  - /air-quality — PM2.5, PM10, ozone, NO2, pollen, UV index, AQI
  - /marine — wave height, period, direction, sea surface temp
  - /elevation — digital elevation data
  - /flood — river discharge and flood forecasts
  - /climate — IPCC warming scenario projections
  - /geocoding — location name → coordinates

### Enrichment: NWS (US-only, no auth, generous)
- Base URL: https://api.weather.gov
- Auth: User-Agent header only (not a key)
- Unique data:
  - **Severe weather alerts** — tornado warnings, severe thunderstorm, flood, winter storm, heat advisory. No other free API provides this.
  - Observation stations — current conditions from real weather stations
  - Zone forecasts — detailed text forecasts by county

## User Personas (for transcendence features)

1. **Morning planner** — "Should I bring an umbrella? Is it safe to bike?" Checks weather before leaving. Wants: current + today's forecast in one glance.
2. **Outdoor event planner** — "Will it rain Saturday for the BBQ?" Wants: multi-day forecast for a specific date.
3. **Runner/cyclist** — "Is it too hot/windy/rainy to run?" Wants: feels-like temp, wind, precipitation chance RIGHT NOW.
4. **Allergy sufferer** — "Is pollen high today?" Wants: air quality + pollen levels.
5. **Parent** — "Is there a severe weather warning? Should I keep kids inside?" Wants: alerts for their area.
6. **Traveler** — "What's the weather like in Tokyo next week vs here?" Wants: compare two locations.
7. **Data nerd** — "Was this summer hotter than average?" Wants: historical comparison.

## Reachability Risk
- **None** — Both APIs public, free, no auth, generous/unlimited limits.

## Top Workflows
1. Quick current conditions + today's forecast
2. Multi-day forecast for trip/event planning
3. Severe weather alerts for my area (NWS)
4. Air quality / pollen / UV check
5. Historical comparison ("is this normal?")

## Table Stakes (from competitors)
- Current conditions + forecast (wttr.in, all weather CLIs)
- Multi-location comparison (wttr.in)
- JSON output (wttr.in format=j1)
- ASCII art weather display (wttr.in)
- Moon phases (wttr.in)
- Hourly breakdown (wttr.in v2)
- Air quality index (open-meteo-mcp)
- Historical data lookup (open-meteo-mcp)
- Marine forecast (open-meteo-mcp)
- Multi-model comparison (open-meteo-mcp: DWD, GFS, ECMWF, etc.)
- Geocoding (open-meteo-mcp)
- Flood forecast (open-meteo-mcp)

## Data Layer
- Primary entities: forecasts, alerts, historical observations, air quality readings
- Sync: current + hourly forecast cached on each lookup (write-through)
- FTS: on location names for quick re-lookup

## Product Thesis
- Name: weather-pp-cli
- Why it should exist: wttr.in is beautiful but curl-only (no local state, no alerts, no history, no --json for agents). Weather MCP servers are AI-only. No CLI combines real-time forecasts with severe weather alerts and 80 years of historical data. This is the first weather CLI that answers "should I go outside?" AND "is it dangerous?" AND "is this normal for April?" in one tool.

## Build Priorities
1. Open-Meteo forecast + geocoding — current conditions, hourly, daily
2. NWS alerts — severe weather warnings for US locations
3. Air quality / pollen / UV
4. Historical comparison — "is this hotter than average?"
5. Transcendence — persona-driven features
