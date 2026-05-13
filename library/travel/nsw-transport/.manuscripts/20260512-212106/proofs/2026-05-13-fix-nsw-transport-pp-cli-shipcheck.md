# nsw-transport-pp-cli — Shipcheck

## Result (after one fix loop)
| Leg | Result |
|---|---|
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| validate-narrative | PASS (after fixing the `trip departures --count 5` quickstart line — `departure_mon` has no count param — and re-pointing two recipe `--select` paths at the actual output shapes) |
| scorecard | PASS — **74/100, Grade B** |

Verdict: **PASS (6/6 legs)**.

## Scorecard breakdown
Output Modes 10, Auth 10, Error Handling 10, Terminal UX 9, README 8, Doctor 10, Agent Native 10, MCP Quality 10, MCP Token Efficiency 7, MCP Remote Transport 5, MCP Tool Design 5, Local Cache 10, Cache Freshness 5, Breadth 7, Vision 8, Workflows 6, Insight 4, Agent Workflow 9. Domain Correctness: Auth Protocol 10, Data Pipeline Integrity 7, Sync Correctness 10, Type Fidelity 3/5, Dead Code 5/5, **Path Validity 0/10**.

## Known gaps (not blockers)
- **Path Validity 0/10 and the 0/7 live sample probe** are artifacts of running shipcheck with no API key. Every NSW endpoint requires either the Open Data Hub key (`NSW_OPENDATA_API_KEY`) or FuelCheck credentials, so unauthenticated probes return 401 and the path-validity check can't distinguish "path exists, auth required" from "path invalid". Not a code defect — to be re-confirmed in Phase 5 if a key is provided.
- **Insight 4/10, Workflows 6/10** — scorecard's view of cross-source depth; the 7 transcendence commands exist and work, but the generated `workflow` command has no registered multi-step workflows and there are no `mcp.intents`. Candidate for polish.
- `fuel trends` is wired against `/v1/fuel/prices/trends` but the endpoint shape is portal-documented, not community-validated — surfaces raw JSON if the shape differs at runtime.

## Fixes applied this loop
1. `research.json` narrative: `trip departures 10101100 --count 5` → `trip departures 10101100` (no `--count` param exists on `departure_mon`).
2. `research.json` recipes: re-pointed the `disruptions` and `commute` `--select` paths at the actual flat-array / nested-array output shapes.
3. Removed dead code (`maxDistKm` const, `roadNames` method) flagged during cleanup.

## Verdict: ship
All ship-threshold conditions met (shipcheck PASS, verify PASS, dogfood wiring clean, workflow-verify pass, verify-skill 0 findings, scorecard 74 ≥ 65). No known functional bugs in shipping-scope features. The 7 transcendence commands plus the realtime & fuel sources all build, pass tests, and dry-run cleanly; live behaviour pending a key in Phase 5.
