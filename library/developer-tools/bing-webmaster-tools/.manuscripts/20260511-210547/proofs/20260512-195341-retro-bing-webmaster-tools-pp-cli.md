# Printing Press Retro: bing-webmaster-tools

## Session Stats
- API: bing-webmaster-tools (Bing Webmaster Tools API)
- Spec source: hand-authored internal YAML (docs-derived — Microsoft Learn `IWebmasterApi` reference cross-checked against the `isiahw1` MCP server, `merj`/`seo-meow`/`charlesnagy`/`webjeyros` community clients, and the `NmadeleiDev/bing_webmaster_cli` Go CLI)
- Scorecard: 82/100 (A)
- Verify pass rate: 100%
- Fix loops: 2 (one in shipcheck — `submit smart` dry-run-before-file-read; one in live dogfood — `traffic ctr-gaps` discarding low-impression queries)
- Manual code edits to *generated* files: 2 (`internal/client/client.go` envelope unwrap + WCF-date normalization; `internal/cli/root.go` one line to register hand-built commands)
- New hand-authored files: 10 (7 transcendence/novel command files + `novel.go` registration shim + `novel_helpers.go` + `url_check.go`)
- Features built from scratch: 7 (`sites health`, `sites triage`, `traffic ctr-gaps`, `keywords cannibalization`, `submit smart`, `crawl triage`, `url check`)

## Findings

### 1. `lock promote` panics with `nil map` assignment when run a second time (Bug)
- **What happened:** The first `printing-press lock promote --cli bing-webmaster-tools-pp-cli --dir <workdir>` succeeded and wrote `state.json`. A second `lock promote` (needed to ship a follow-up live-test fix into the library) panicked: `panic: assignment to entry in nil map` at `internal/pipeline/state.go:510`, via `LoadState` ← `NewMinimalState` ← `internal/cli/lock.go:238`.
- **Scorer correct?** N/A (not a score-penalty finding).
- **Root cause:** Component: the binary's pipeline-state code — `internal/pipeline/state.go`, `LoadState`. The first `lock promote` rewrites `state.json` with `working_dir` pointed at the *library* path (not the original working dir) and `phases: null`. A subsequent `lock promote --dir <original-workdir>` no longer matches via `FindStateByWorkingDir`, so `lock.go:238` falls back to `NewMinimalState`, which calls `LoadState(apiName)`. `LoadState` unmarshals the on-disk `state.json` → `Version: 0` (no/zero `version`), `Phases: nil` (from `phases: null`). Because `0 < currentStateVersion (3)`, the migration loop at lines ~505-525 does `s.Phases[name] = PhaseState{...}` for each `name` in `PhaseOrder` — assigning into a nil map → panic. `NewMinimalState` itself initializes `Phases` to a non-nil map, but the `LoadState` path it then calls does not, and the migration loop runs on the loaded (nil-map) state.
- **Cross-API check:** Recurs for *any* CLI — the panic is not API-specific. It triggers on the second `lock promote` for a given CLI (e.g., promote → polish a fix → re-promote; or the SKILL's own Phase 6 hold-path which re-runs `lock promote` after a "Polish to retry" pass).
- **Frequency:** every CLI that gets re-promoted (a documented flow in the main SKILL's Phase 6 hold-path and a natural flow after a post-promote live-test fix).
- **Fallback if the Printing Press doesn't fix it:** The agent copies the changed file(s) into the library directory by hand and rebuilds the binary there (what this run did). Loses the manifest/CurrentRunPointer update that `lock promote` performs; brittle; easy to forget.
- **Worth a Printing Press fix?** Yes — a reproducible panic that aborts a documented flow, with a ~1-line fix.
- **Inherent or fixable:** Fixable. In `LoadState`, after unmarshal and before the version-migration loop: `if s.Phases == nil { s.Phases = make(map[string]PhaseState) }`. (Belt-and-suspenders: also guard the migration loop, and consider having `PromoteWorkingCLI`/`lock promote` write a `version`-stamped state.json with `phases: {}` rather than `phases: null`.)
- **Durable fix:** Template/binary fix in `internal/pipeline/state.go` — initialize `Phases` to a non-nil map in `LoadState` before any code path that writes into it. Plus, in `lock promote`'s state-rewrite path, persist `version: currentStateVersion` and `phases: {}` instead of `phases: null`.
- **Test:** Positive — write a `state.json` with `{"version":0,"phases":null,...}`, call `lock promote --dir <dir>` twice; both succeed, no panic. Negative — a valid current-version `state.json` with populated `phases` still promotes correctly and the migration loop is a no-op.
- **Evidence:** Two stack traces in the session: first on a `lock acquire`/`lock promote` re-run, then again on a verbose `lock promote` retry — both `panic: assignment to entry in nil map` at `internal/pipeline/state.go:510`. The on-disk `state.json` at that point: `{"version":0, ..., "working_dir":".../library/bing-webmaster-tools", "phases":null, ...}`.
- **Related prior retros:** None (no prior retro files found under `~/printing-press/manuscripts/*/proofs/`).
- **Step G (case against filing):** "Re-running `lock promote` directly is an unusual flow; the user should re-run the full pipeline." — Fails: the main SKILL's own Phase 6 hold-path re-invokes `lock promote` after a "Polish to retry" pass, and shipping a post-promote fix into the library is a natural workflow; a panic is never "works as designed," and the fix is trivial. Case-for clearly stronger.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | `lock promote` panics (`nil map`) on second run | generator (`internal/pipeline/state.go`) | every re-promoted CLI | low (manual file copy + rebuild, easy to forget) | small | none — pure defensive init |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| A | Spec-level response-envelope unwrap + legacy date normalization | Step B: real pattern (WCF `.svc` JSON `{"d":...}` envelope, OData v2, SharePoint REST) but can name only one *catalog* API with concrete evidence (this one); `response_path` already exists per-endpoint, so the gap is "no spec-level default + handle object/null payloads, not just arrays" — a genuine but narrow improvement. Step G case-against (agent could set `response_path: d` per-endpoint or write a small spec loop; a vendor-specific response transform in `client.go` is borderline-normal per-CLI customization) is roughly even → default direction is don't-file. Reconsider if a second enveloped-JSON API surfaces. |
| C | `sync` can't fan a required parent param (e.g., `siteUrl`) across values | Step B: the "parent-scoped list endpoint" pattern is broad but I can name only ~2 catalog APIs with concrete evidence; it's a known-hard problem (the generator emits `sync` for top-level list endpoints, not parent-scoped ones), and the workaround is "the analytics commands work live." Step G case-against (this is inherent to spec-driven `sync` without parent-discovery choreography; belongs in the SKILL as a recipe if anywhere) is at least as strong as the case-for. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Store "no extractable ID field found" for `Url`-keyed resources | `sites`/`traffic` rows skipped on sync because the key is `Url` not `Id` | printed-CLI — internal specs already have an `id_field:` knob (`internal/spec/spec.go:941`); I just didn't set it. |
| No first-class extension point for hand-built novel commands | Had to add one line to `root.go` (DO NOT EDIT) + a `novel.go` shim | printed-CLI / instructional — the main SKILL's Phase 3 template already documents editing `root.go` to register novel commands as the workflow. |
| `Get<Resource>Info` paths → `info` endpoint verb fails dogfood's read-verb rule | polish renamed `url info` → `url get` | printed-CLI — for internal specs the command name comes from the endpoint key the *author* chose (`info`), not the path; naming it `get` from the start avoids it. Already resolved by polish. |
| MCP `>50 tools` required adding the `mcp:` block by hand | Regenerated with `mcp: orchestration: code` after the generator's warning | API-quirk / worked-as-designed — the generator printed the exact `mcp:` block to add and the SKILL's Pre-Generation MCP Enrichment step covers it; no friction beyond a 5-line edit + regen. |
| `submit smart` read `--file` before the dry-run short-circuit | `--dry-run` with a non-existent file path exited 2 | iteration-noise — bug in *my own* hand-authored novel command, fixed in-session; not generator output. |
| `traffic ctr-gaps` discarded low-impression queries (`int(lostClicks)==0`) | returned `[]` for a low-traffic site | iteration-noise — bug in *my own* hand-authored novel command, fixed in-session during live dogfood; not generator output. |

## Work Units

### WU-1: `LoadState` must initialize `Phases` before the version-migration loop (from F1)
- **Priority:** P2
- **Component:** generator (the fix lands in `internal/pipeline/state.go`; `internal/pipeline` is generation-pipeline infrastructure — no closer slug exists in the fixed set)
- **Goal:** `printing-press lock promote` never panics with `assignment to entry in nil map`, regardless of how many times it's run for a CLI.
- **Target:** `internal/pipeline/state.go` (`LoadState` — the unmarshal-then-migrate path, around lines ~480-525), and the state-write path in `pipeline.PromoteWorkingCLI` / `internal/cli/lock.go`'s `newLockPromoteCmd`.
- **Acceptance criteria:**
  - positive test: a `state.json` containing `{"version":0,"phases":null,...}` (or no `version`/`phases` keys) loads without panic; `lock promote --dir <dir>` run twice in a row both succeed.
  - negative test: a current-version `state.json` with a populated `phases` map still loads and promotes correctly; the migration loop is a no-op and doesn't clobber existing phase state.
- **Scope boundary:** Does not change the state-file schema or the promote semantics — only ensures `Phases` is a non-nil map before anything writes into it, and (optionally) makes `lock promote` persist `version: currentStateVersion` + `phases: {}` rather than `phases: null`.
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns
- Filing the response-envelope and `sync`-fan-out findings at P2/P3 anyway "because the friction was real" — the friction was real for *this* CLI, but Step B couldn't name three catalog APIs with concrete evidence for either, so they go to Skip, not Do. (One sharp P2 bug beats three speculative findings.)

## What the Printing Press Got Right
- The `>50 MCP tools` warning fired with the *exact* `mcp:` block to add — a one-paste fix, no guesswork. The Pre-Generation MCP Enrichment step in the SKILL did its job.
- The internal-YAML spec format handled a 56-method, 13-resource API cleanly: `api_key`-in-query auth, `verify_path` for `doctor`, `flag_name`/`aliases` for the cryptic `q` keyword param, GET-with-params vs POST-with-body — all generated correctly on the first pass; all 8 quality gates green.
- `shipcheck` / `verify-skill` / `validate-narrative` caught every stale reference between the (pre-update) generated README/SKILL and the corrected `research.json` narrative — `--site` vs `--site-url`, `--all-sites`/`--days` flags that didn't exist, `site health` vs `sites health` — so a regen + the two re-applied hand-edits produced a consistent set. The "regenerate after fixing `research.json`" loop worked.
- The polish skill's output-review caught a genuine UX bug — `sites health` reporting a confident `F`/score-0 when the backing endpoints had actually all failed — and the fix (`dataComplete` + suppress the grade) shipped before the user ever saw it.
- The agent-native surface (`--json`, `--select` dotted paths, `--compact`, `--dry-run`, typed exit codes, `cliutil.IsVerifyEnv()` short-circuit, `dryRunOK`) was all there for the hand-built novel commands to lean on — they got the full toolkit for free.
