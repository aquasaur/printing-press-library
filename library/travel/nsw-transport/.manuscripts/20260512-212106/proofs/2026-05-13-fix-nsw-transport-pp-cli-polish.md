# nsw-transport-pp-cli — Polish Pass

| Metric | Before | After |
|---|---|---|
| Scorecard | 74/100 | 75/100 (+1) |
| Verify | 100% | 100% |
| Dogfood | PASS | PASS |
| go vet | 0 | 0 |
| Tools-audit | 0 pending | 0 pending |
| Publish validate | FAIL | **PASS** (3 checks fixed) |

## Fixes applied
- `go mod tidy` — go.sum was missing entries.
- `printing-press mcp-sync` — regenerated the missing `tools-manifest.json`; gofmt on `internal/mcp/tools.go`.
- Copied `phase5-acceptance.json` + the build/acceptance/shipcheck proof markdown into `<cli>/.manuscripts/<run-id>/proofs/` so publish-validate's phase5 check passes.
- Added `htmlToPlainText()` in `internal/cli/nsw_helpers.go`; applied to `hazardFeature.OtherAdvice`/`.Headline` so `commute --agent`/`--json` no longer embed `<p>...</p>` HTML inside JSON string fields (Phase 4.85 finding).

## Skipped / deferred (non-blocking)
- Verify WARN "sync crashed" — mock-mode artifact: the harness feeds the `carpark` resource fake items with no `tsn` field, so 0 are stored and the harness misreads it as a crash. The `carpark`→`tsn` ID-field override is correct for the real API. verify pass_rate is 100%.
- Verify execute:false on `cameras`/`hazards`/`refresh` — `hazards` requires positionals (exit 2 without them); `refresh` is a `pp:client-call` helper that prints "FuelCheck credentials not configured" (exit 0, no probe-string match); `cameras` actually runs fine. Harness false negatives.
- Scorecard `path_validity 0/10` — internal-YAML spec validates paths at parse time; the dimension has no runtime signal. Structural.
- Scorecard `mcp_remote_transport 5/10` + `mcp_tool_design 5/10` — closable by adding `mcp.transport: [stdio, http]` + `mcp.intents` to the spec and regenerating; deferred (regen mid-pipeline risks the working CLI). Follow-up candidate.
- Scorecard `insight 4/10` — no agent-grade improvement available without adding scaffolding.
- `disruptions --all` ranks interstate (Melbourne/Brisbane) alerts to the top — data is correct (NSW TrainLink regional feeds include interstate terminus alerts); a `--region`/`--nsw-only` filter would be a feature add, out of polish scope. (Retro/follow-up candidate.)

## Ship recommendation: **ship** (`further_polish_recommended: no`)
All hard ship-gates pass: dogfood PASS, verify 100%, verify-skill 0 findings, workflow-verify workflow-pass, publish-validate PASS, tools-audit clean.
