# Shipcheck — bing-webmaster-tools-pp-cli

## Result: `ship`

`printing-press shipcheck` exit 0 — **6/6 legs PASS**.

| Leg | Result |
|---|---|
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS (`workflow-pass`) |
| verify-skill | PASS (SKILL flags/commands all resolve) |
| validate-narrative | PASS (9 ok, 0 missing, 0 failed examples after fixing `submit smart`'s dry-run-before-file-read) |
| scorecard | PASS |

## Scorecard: 80/100 — Grade A

Strong (10/10): Output Modes, Auth, Error Handling, Doctor, Agent Native, MCP Remote Transport, MCP Tool Design, MCP Surface Strategy, Local Cache, Breadth, Path Validity, Sync Correctness.
Mid (8–9): Terminal UX 9, README 8, MCP Quality 8, Agent Workflow 9.
Polish candidates: Insight 2/10, Cache Freshness 5/10, Workflows 6/10, Vision 7/10, Auth Protocol 4/10, Data Pipeline Integrity 7/10, Type Fidelity 3/5, Dead Code 4/5.

## Sample Output Probe: 2/6
The 4 "failures" are all `GET /Get… returned HTTP 400: {"ErrorCode":3,"Message":"ERROR!!! InvalidApiKey"}` — the expected response when no real API key is configured. The novel-feature commands correctly call the live API and surface a clean auth error rather than wrong/empty output. No flagship feature returns incorrect data.

## Fixes applied this round
- `submit smart`: moved the dry-run / verify-env short-circuit ahead of the `--file` read so `--dry-run` examples with a non-existent file path exit 0 (was exit 2 → validate-narrative failure).
- Regenerated the CLI from the corrected `research.json` narrative so README/SKILL Quick Start, recipes, and value-prop match the actual command surface (`sites health`/`sites triage`, `--site-url` not `--site`, no `--all-sites`/`--days` flags that don't exist). Re-applied the two hand-edits (`client.go` envelope unwrap, `root.go` novel-command registration) after regen.

## Behavioral spot-checks (mock / dry-run)
- `sites health|triage --dry-run`, `traffic ctr-gaps --dry-run`, `keywords cannibalization --dry-run`, `crawl triage --dry-run`, `url check --dry-run`, `submit smart --dry-run` (incl. missing `--file`) — all exit 0.
- `sites/traffic/crawl/keywords/submit/url --help` enumerate the novel subcommands.
- `PRINTING_PRESS_VERIFY=1 submit smart --site-url … --url …` reports the plan with `dryRun:true` and never calls the API.
- Live novel-feature invocations hit the real `ssl.bing.com` endpoint and return `InvalidApiKey` (auth wired correctly).

## Ship recommendation: `ship`
All ship-threshold conditions met. Live smoke testing (Phase 5) is pending a real `BING_WEBMASTER_API_KEY` — the API requires auth and none was available in the session, so Phase 5 auto-skips per the skill contract.
