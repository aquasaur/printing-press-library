---
title: "feat: Make PP scrape-creators CLI adoption-ready as the official @scrapecreators/cli"
type: feat
status: active
date: 2026-04-23
---

# feat: Make PP scrape-creators CLI adoption-ready as the official @scrapecreators/cli

## Overview

Two CLIs exist today for the same API.

1. `@scrapecreators/cli` v1.0.0 on npm, maintained by Adrian Horning (ScrapeCreators). Node, commander-based, 110+ endpoints, clean `platform action` naming, interactive wizard, hosted MCP, `agent add` auto-wiring.
2. Printing Press PR #113 under `library/developer-tools/scrape-creators/` on `mvanhorn/printing-press-library`. Go, cobra-based, the same 110+ endpoints, plus a local SQLite sync layer, agent-first flag surface, compound workflows, a local MCP companion binary, a doctor command, and eight unique analytics capabilities that are not in any other tool for this API.

The goal is to close the gap so Adrian can adopt the PP build and publish it as `@scrapecreators/cli` v2. The plan lists (a) what PP already delivers that the existing CLI does not, (b) the concrete handoff blockers that remain, and (c) implementation units that close those blockers.

## Problem Frame

The existing `@scrapecreators/cli` is a thin imperative wrapper: one call per endpoint, pretty JSON out, optional interactive wizard, no local persistence, no analytics, no compound workflows, no MCP binary. It is a competent but minimal CLI.

PP PR #113 already ships the surface Adrian needs next: agent-first flag conventions, structured exit codes, local sync + FTS + analytics, a companion MCP binary, dry-run, rate limit, response cache, progress events, and eight analytics superpowers (videos spikes, transcripts search, profile compare, videos cadence, profile track, account budget, search trends, videos analyze). All of these are unique capabilities for this API.

What is not yet adoption-ready:

1. Binary name still carries the `-pp-` infix (`scrape-creators-pp-cli`).
2. Command names come straight from OpenAPI operation IDs (`list`, `list-post-2`, `list-user-5`, `list-adlibrary-3`). Adrian's CLI reads `instagram profile`, `instagram user-posts`, `facebook adlibrary-search-ads`, with `/v1` vs `/v3` versions collapsed by `parseToolPath`. The PP naming is the single biggest adoption blocker.
3. Node users expect `npm i -g @scrapecreators/cli` and `npx @scrapecreators/cli`. PP today only ships `go install` and homebrew via goreleaser.
4. Adrian's CLI has `scrapecreators agent add cursor|claude|codex` auto-wiring. PP asks users to hand-edit JSON.
5. Adrian's CLI launches an interactive wizard on no-args (@clack/prompts). PP has no wizard; its "agent-first" posture means bare invocation just prints help.
6. Top-of-README input normalization guidance (handles without `@`, hashtags without `#`, transcript 2-minute limit) is prominent on Adrian's README and scattered in PP's per-endpoint descriptions.
7. Brand: PP's README links to `scrapecreators.com` but is hosted under `mvanhorn/printing-press-library`. Adoption requires either moving the source to `ScrapeCreators/scrapecreators-cli` or publishing from a tag in the PP repo into the `@scrapecreators/cli` npm namespace under Adrian's account.

## Requirements Trace

- R1. `@scrapecreators/cli` v2 installs via `npm i -g @scrapecreators/cli` and runs via `npx @scrapecreators/cli` on macOS, Linux, and Windows.
- R2. Binary is named `scrapecreators` (no `-pp-` infix) across npm, brew, and GitHub Releases.
- R3. Every endpoint reachable in v1 (110+ endpoints, version-deduped) is reachable in v2 under a human-readable `platform action` name that matches or reasonably migrates from v1's naming. A migration table documents any v1 -> v2 renames.
- R4. Every capability that is unique to PP (sync, search, analytics, tail, export, import, workflow, doctor, account usage breakdowns, and the eight superpowers) is preserved in v2.
- R5. `scrapecreators agent add cursor|claude|codex|claude-desktop` wires up both the hosted MCP (via `api.scrapecreators.com/mcp`) and the local MCP binary (`scrapecreators-mcp`) in one command.
- R6. Bare `scrapecreators` with no args launches an interactive wizard equivalent to v1's @clack/prompts flow. `--no-input`, `--agent`, or any subcommand disables the wizard.
- R7. Agent-first flags keep v1 parity by default: `--json` (default in non-TTY), `--pretty`, `--format table|csv|markdown`, `--output`, `--clean` (alias for `--compact`). PP-unique flags (`--select`, `--agent`, `--dry-run`, `--rate-limit`, `--no-cache`, `--yes`, `--stdin`, `--no-input`, `--human-friendly`) are retained.
- R8. Exit codes are documented (`0/2/3/4/5/7/10`) and stable across v1-compatible commands. v1 had no documented exit codes, so this is net new but non-breaking.
- R9. README leads with Adrian's audience: install, API key, first command, input normalization (no `@`, no `#`, 2-min transcript cap), then output formats, then unique-to-v2 features (local sync and superpowers), then MCP, then agent integration, then troubleshooting.
- R10. A dry-run migration guide tells v1 users how old commands map to v2 commands, which flags changed, and what is new.
- R11. Test coverage exists for the security-sensitive paths v1 covers in `security.test.js` (URL validation, API key redaction, injection resistance) plus a smoke test for every registered `platform action` subcommand.

## Scope Boundaries

In scope:

- Source modifications inside `library/developer-tools/scrape-creators/` on PR #113 to close handoff blockers.
- A new npm distribution shell that wraps the Go binary with platform-dispatch (mirrors Adrian's v1 install surface).
- Command-name remapping from OpenAPI operation IDs to human-readable actions.
- Migration guide and README restructure.
- Agent auto-wiring command.
- Interactive wizard on bare invocation.

### Deferred to Follow-Up Work

- Transferring the published repo home from `mvanhorn/printing-press-library` to `ScrapeCreators/scrapecreators-cli` (or setting up a mirror). This is Adrian's org-level decision, not a code change in this plan.
- Publishing to Adrian's npm namespace. Requires the `@scrapecreators` npm scope owner (Adrian) to either publish or grant publish rights. This plan produces the npm artifacts; Adrian decides how they ship.
- Retiring v1. Adrian decides when to deprecate the commander-based code.
- Rewriting the generator (cli-printing-press) to produce human-readable action names for all CLIs. This plan hand-maps scrape-creators only; a generator-level fix is a separate effort.

Out of scope (explicit non-goals):

- Rebuilding Adrian's commander-based v1 from scratch. The PP Go build is the baseline.
- Changing the underlying API. No endpoint additions, removals, or schema changes.
- Introducing a hosted web dashboard or account management UI.
- Breaking v1's output shape. v2 must accept v1-equivalent flags and return v1-equivalent JSON for shared endpoints.

## Context & Research

### Evidence: what PP already has that v1 does not

Distribution and runtime:

| Dimension | v1 (`@scrapecreators/cli`) | PP PR #113 |
|-----------|----------------------------|------------|
| Runtime | Node.js >= 20 | Go, single static binary |
| Startup | Node + commander resolve | near-zero cold start |
| Dependencies | @clack/prompts, chalk, cli-table3, commander, conf, ora | none at runtime |
| Binary | `scrapecreators` | `scrape-creators-pp-cli` + `scrape-creators-pp-mcp` |

Flag surface (agent usefulness):

| Capability | v1 | PP |
|------------|----|----|
| JSON output | default (compact) + `--pretty` | `--json` + `--compact` |
| Field projection | `--clean` (predefined strip) | `--select field,list` (user-chosen) |
| Table / CSV / markdown | `--format table\|csv\|markdown` | `--human-friendly` (table), `--csv`, `--plain`, `--quiet` |
| Dry run | none | `--dry-run` |
| Rate limit | none | `--rate-limit` |
| Response cache | none | 5-min GET cache, `--no-cache` |
| Stdin piped input | none | `--stdin` (docs) |
| Timeout | none documented | `--timeout` |
| Confirmation gating | none documented | `--yes` |
| Non-interactive enforcement | implicit via no-TTY | explicit `--no-input` |
| Agent bundle | none | `--agent` (= json + compact + no-input + no-color) |
| Exit codes | undocumented | `0/2/3/4/5/7/10` documented |
| Progress telemetry | none | NDJSON events to stderr during pagination |

Local data layer (PP-unique):

| Command | What it does |
|---------|--------------|
| `sync` | pull API data into local SQLite (`~/.config/scrape-creators-pp-cli/data.db`) |
| `search` | FTS across synced data, falls back to live API with `--data-source live` |
| `analytics` | run aggregate queries on the local DB (counts, trends, engagement rollups) |
| `tail` | poll the API and stream changes as NDJSON |
| `export` | dump synced data to JSONL or JSON |
| `import` | load JSONL back into local DB |
| `workflow` | run compound multi-operation recipes |
| `doctor` | end-to-end health check (config, auth, reachability, DB integrity) |

Superpowers (unique analytics, all local-data-backed):

| Command | Claim |
|---------|-------|
| `videos spikes` | find videos significantly above a creator's average |
| `transcripts search` | grep across a creator's video transcripts |
| `profile compare` | side-by-side follower count, engagement rate, cadence |
| `videos cadence` | post schedule by day of week and hour |
| `profile track` | daily follower snapshots and growth charts |
| `account budget` | credit spend tracking and days-until-limit projection |
| `search trends` | hashtag growth via snapshot diffs |
| `videos analyze` | engagement-rate rank of all synced videos for a creator |

MCP:

| Dimension | v1 | PP |
|-----------|----|----|
| Local MCP binary | no | yes (`scrape-creators-pp-mcp`) |
| Hosted MCP URL | yes (`api.scrapecreators.com/mcp`) | no (must re-add for parity) |
| Auto-configure client | `scrapecreators agent add cursor\|claude\|codex` | manual JSON in README |

### Evidence: what v1 has that PP lacks

1. Clean command naming. v1's `command-registry.js` maps OpenAPI paths to `platform action` pairs via `parseToolPath()`, deduping `/v1` vs `/v3` and picking the latest. Examples: `/v3/tiktok/profile/videos` -> `tiktok profile-videos`, `/v1/linktree` -> `linktree get`, `/v1/detect-age-gender` -> `detect age-gender`. PP today surfaces every operation ID as-is: `list`, `list-post`, `list-post-2`, `list-user-5`, `list-adlibrary-3`. A new user cannot guess the command; existing v1 users would not recognize any of it.

2. npm install. v1 is `npm i -g @scrapecreators/cli` and `npx @scrapecreators/cli`. PP is `go install` or a goreleaser binary. Adrian's users are predominantly Node and no-code audiences; requiring Go is a blocker.

3. Interactive wizard. v1's `interactive.js` uses @clack/prompts to walk a user through platform -> action -> params when invoked bare. PP's bare invocation prints help.

4. `agent add <target>` auto-wiring. v1 ships a single command that writes MCP config for Cursor, Claude Desktop, or Codex. PP documents manual JSON edits.

5. Input normalization surfaced up front. v1 README has a dedicated block (no `@`, no `#`, transcript 2-min cap). PP scatters this detail across per-endpoint help text.

6. Agent skills package. v1 offers `npx skills add scrapecreators/agent-skills` to install skills into Cursor, Claude Code, Codex standalone. PP's SKILL.md ships inside the printing-press-library Claude Code plugin, which is a different distribution channel.

7. Brand alignment. v1 is `@scrapecreators/cli` under the `ScrapeCreators` GitHub org with `scrapecreators.com` homepage. PP is under `mvanhorn/printing-press-library` with a generated binary name that reads `scrape-creators-pp-cli`.

### Institutional Learnings

- `docs/plans/2026-04-19-001-feat-ppl-brew-of-clis-and-distribution-plan.md` already defines a scoped `@printing-press/*` NPM namespace with a platform-dispatch wrapper. That wrapper pattern transfers directly to `@scrapecreators/cli` v2; only the scope and binary name change.
- `docs/solutions/` pattern: whenever a generated name would be shown to a human, a curated override layer wins. The Printing Press generator's direct-OpenAPI naming is acceptable for agent-only CLIs but becomes an adoption blocker the moment a human audience is primary.
- `AGENTS.md` rule: `.printing-press.json` is the source of truth for a CLI's identity. Any binary rename has to update `cli_name` there, and any regenerated SKILL.md has to be committed to `skills/pp-scrape-creators/SKILL.md` alongside the library change.

### External References

- v1 source (for parity): `github.com/ScrapeCreators/scrapecreators-cli`, files `src/command-registry.js` (operation-ID -> platform/action), `src/interactive.js` (wizard), `bin/scrapecreators.js` (entry), `test/security.test.js` (13,606 bytes of security assertions).
- v1 README: install, auth hierarchy, platform/action pattern, discovery (`list`, `list <platform>`), output formatting table, input normalization, hosted MCP, agent skills.
- v1 package.json: Node >= 20, dependencies pinned (commander 13.1.0, conf 13.1.0, chalk 5.6.2, ora 8.2.0, cli-table3 0.6.5, @clack/prompts 0.10.1), vitest 4.1.4 for tests.

## Key Technical Decisions

- Keep the Go implementation as the v2 base. The local SQLite sync, zero-dep static binary, and startup latency are worth more than the cost of porting to Node.
- Distribute via npm through a platform-dispatch wrapper (pattern already spec'd in `docs/plans/2026-04-19-001-feat-ppl-brew-of-clis-and-distribution-plan.md`). The npm package runs the correct prebuilt Go binary for the user's platform with no postinstall network calls. Same pattern esbuild uses.
- Bind the binary name to `scrapecreators` across all channels. Drop the `-pp-` infix for this CLI only; the convention stays elsewhere in the Printing Press library.
- Build a hand-maintained `action_map.yaml` to rename operation-ID names to v1-style `platform action` names. Prefer v1's existing names wherever they already cover an endpoint, so existing users' muscle memory transfers. For endpoints v1 does not expose, invent new names following v1's pattern (no `list-` prefix, no `-N` version suffix, version dedupe baked in).
- Keep the TOML config file at `~/.config/scrape-creators-pp-cli/config.toml` during transition, but also accept `~/.config/scrapecreators/config.toml` (v1's path). On first run, migrate from v1's path if present.
- Preserve every PP flag. For v1 compatibility, accept `--pretty` (alias `--json` without `--compact`), `--format table|csv|markdown` (alias for `--human-friendly` / `--csv` / dedicated markdown formatter), and `--clean` (alias for `--compact`).
- Ship one MCP artifact that supports both modes: runs as a local stdio server by default, connects to `api.scrapecreators.com/mcp` when configured for hosted. `scrapecreators agent add <target>` writes whichever the user picks.
- Tests: port the four test suites from v1's `security.test.js` (URL sanitization, key redaction, injection, path traversal) to Go `testing` + a generator that walks the registered cobra command tree and asserts every subcommand has a `--help` block, produces valid JSON on `--dry-run`, and refuses unknown fields on `--select`.

## Open Questions

### Resolved During Planning

- Should the PP sync layer ship in v2 or be deferred? Ship. It is the single biggest reason Adrian would adopt PP instead of iterating on v1.
- Should v2 include the eight superpowers? Yes. They are the differentiation that justifies the version bump and is the ceiling Adrian cannot reach with his current commander-based shape.
- Should v2 keep cobra's help surface or emulate commander's? Keep cobra. Its `--help` trees and autocompletion are strictly better than commander's. The visible UX (command discovery, flag semantics, JSON output) is what migrates from v1, not the help renderer.

### Deferred to Implementation

- Exact migration behavior when both `~/.config/scrapecreators/config.toml` (v1) and `~/.config/scrape-creators-pp-cli/config.toml` (PP) exist. Prefer v1's on first run, warn, copy forward. Finalize exact conflict rule at implementation time.
- Whether to embed the OpenAPI spec inside the binary or keep it external. Embedding simplifies offline `api` browsing; external keeps binary size lower. Decide once binary size is measured.
- How the wizard handles pagination flags (`cursor`, `after`, `max_id`, `next_max_id` vary per platform). v1 skips pagination in the wizard. Decide whether to expose it in v2.
- The exact action-map entries for the ~30 endpoints v1 does not expose (the promoted_* family, the `list-getapiusage` analytics endpoints, the workflow command). These need endpoint-by-endpoint naming calls at implementation time.

## Implementation Units

- [ ] U1. Rename binary and brand

Goal: `scrapecreators` becomes the single binary name across npm, homebrew, GitHub Releases, and MCP configuration examples. The `-pp-` infix is dropped for this CLI only.

Requirements: R2.

Dependencies: none.

Files:
- Modify: `library/developer-tools/scrape-creators/.printing-press.json` (update `cli_name` to `scrapecreators`; add `cli_alias: ["scrape-creators-pp-cli"]` for discoverability)
- Modify: `library/developer-tools/scrape-creators/.goreleaser.yaml` (binary name, homebrew formula name, archive naming)
- Rename: `cmd/scrape-creators-pp-cli/` -> `cmd/scrapecreators/`
- Rename: `cmd/scrape-creators-pp-mcp/` -> `cmd/scrapecreators-mcp/`
- Modify: `internal/cli/root.go` (`Use:` string)
- Modify: `README.md` (every reference, install lines, MCP config examples, env var prefix stays `SCRAPE_CREATORS_API_KEY_AUTH` because that is v1-compatible)
- Modify: `Makefile`
- Modify: `skills/pp-scrape-creators/SKILL.md` if present (regenerate)

Approach:
- The `-pp-` infix is a convention across the Printing Press library to avoid collisions with official vendor CLIs. This CLI is being handed to the official vendor, so the collision stops being a concern here. Mark the exception in `AGENTS.md` or `.printing-press.json` so future regenerations do not re-add the infix.
- Keep `SCRAPE_CREATORS_API_KEY_AUTH` unchanged. Changing env var names is a breakage PP v1 users would feel; leave it.

Patterns to follow:
- Binary renaming pattern already used on `dominos-pp-cli` vs newer slug-only directories (see AGENTS.md inconsistency note).
- `.printing-press.json` cli_alias field (if it does not exist yet, add it alongside `cli_name`).

Test scenarios:
- Happy path: `go build ./cmd/scrapecreators` produces a `scrapecreators` binary that responds to `--help` and `--version`.
- Happy path: `scrapecreators-mcp` binary starts and responds to MCP initialize.
- Regression: `go run ./tools/generate-skills/main.go` regenerates `skills/pp-scrape-creators/SKILL.md` without re-adding the `-pp-` infix.
- Regression: `.github/scripts/verify-skill/verify_skill.py` passes against the renamed binary.

Verification:
- `scrapecreators --version` prints a version.
- `scrapecreators-mcp` is a runnable binary.
- No string `scrape-creators-pp-cli` remains in README.md, SKILL.md, or cmd paths.

- [ ] U2. Remap command names to v1-style `platform action`

Goal: Replace the OpenAPI-operation-ID naming (`list`, `list-post`, `list-post-2`, `list-user-5`, `list-adlibrary-3`) with human-readable `platform action` pairs that match or reasonably migrate from v1's names.

Requirements: R3.

Dependencies: U1.

Files:
- Create: `library/developer-tools/scrape-creators/action_map.yaml` (endpoint path -> v2 `platform action` name, plus optional v1 alias list)
- Create: `library/developer-tools/scrape-creators/internal/cli/action_map.go` (compiled lookup table, generated from action_map.yaml at build time or hand-maintained)
- Modify: every file under `internal/cli/<platform>_list*.go` to register commands under mapped action names. Keep operation-ID names as hidden aliases for backward compatibility within this PR's lifetime.
- Create: `library/developer-tools/scrape-creators/MIGRATION.md` (v1 -> v2 command map, alias policy)

Approach:
- Write the action map by hand. Do not auto-generate from OpenAPI. Names are a human-audience decision that the generator is not qualified to make.
- Start the map by lifting every `platform action` pair v1 already exposes via `parseToolPath`. That covers the endpoint subset v1 supports. Extend with names for PP-only endpoints (promoted_*, account analytics endpoints, workflow, tail, etc.) using v1's pattern (lowercase, hyphen-separated, no version suffix).
- Register each cobra command twice: once under the v2 name (primary, shown in help) and once as a hidden alias under the legacy operation-ID name (`cobra.Command{Aliases: [...], Hidden: true}` at the alias level). Hidden aliases let scripts that already depend on the old names keep working for one major version.
- Deduplicate `/v1` vs `/v3` the way `parseToolPath` does: prefer the latest version. Add a regression test asserting every `platform action` pair points to exactly one endpoint.

Patterns to follow:
- v1's `parseToolPath` in `src/command-registry.js` for the path-to-action algorithm.
- cobra's `Aliases` field and `Hidden: true` for soft deprecation.

Test scenarios:
- Happy path: `scrapecreators tiktok profile charlidamelio --dry-run` resolves to the `/v3/tiktok/profile` endpoint.
- Happy path: `scrapecreators instagram user-posts <handle> --dry-run` resolves to the v3 reels+posts endpoint.
- Regression: every v1 command from `command-registry.js` parses and resolves to the same endpoint in v2.
- Regression: operation-ID aliases (`list-post-2`, `list-user-5`) still resolve but do not appear in `--help` output.
- Edge case: platforms with one endpoint (`linktree`, `komi`, `pillar`, `linkbio`) expose `<platform> get` per v1.
- Edge case: `detect-age-gender` parses as a single-platform-with-hyphen per v1's special case.
- Error path: an unknown `platform action` pair prints a suggestion (`did you mean "tiktok profile"?`) using cobra's built-in suggestion engine.

Verification:
- Every entry in action_map.yaml maps to a registered cobra command and exactly one endpoint.
- Every `platform action` pair v1 exposes resolves identically in v2.

- [ ] U3. npm distribution wrapper as `@scrapecreators/cli` v2

Goal: `npm i -g @scrapecreators/cli` and `npx @scrapecreators/cli` invoke the Go binary on macOS, Linux, and Windows with no postinstall network calls.

Requirements: R1.

Dependencies: U1 (binary name stable).

Files:
- Create: `library/developer-tools/scrape-creators/npm/package.json` (scope `@scrapecreators`, name `cli`, version `2.0.0`, `bin.scrapecreators` pointing at `dist/index.js`, optionalDependencies for per-platform packages)
- Create: `library/developer-tools/scrape-creators/npm/platform-dispatch.js` (resolves binary path by process.platform + process.arch, execs with passthrough argv)
- Create: `library/developer-tools/scrape-creators/npm/platforms/<os>-<arch>/` directories for each goreleaser target
- Modify: `.goreleaser.yaml` to emit per-platform tarballs the npm build can consume
- Create: `.github/workflows/publish-scrapecreators-npm.yml` (builds Go binary for each platform, packs into npm sub-packages, publishes the root `@scrapecreators/cli` package with optionalDependencies)

Approach:
- Mirror the platform-dispatch pattern already planned for `@printing-press/*` in `docs/plans/2026-04-19-001-feat-ppl-brew-of-clis-and-distribution-plan.md`. Change only the npm scope and the binary name.
- Root package is near-empty: `platform-dispatch.js` and a `bin/` shim. Per-platform packages are declared in `optionalDependencies`. npm installs only the one that matches the user's OS/arch. No postinstall script.
- v1 is currently at 1.0.0. Publish v2 as `@scrapecreators/cli@2.0.0` so `npm i -g @scrapecreators/cli` picks it up automatically for anyone who does not pin.

Patterns to follow:
- esbuild's npm dispatch pattern (as documented in PP's existing brew-of-clis plan).
- goreleaser's `archives:` block already configured per the existing `.goreleaser.yaml`.

Test scenarios:
- Happy path: on macOS arm64, `npm i -g @scrapecreators/cli` installs and `scrapecreators --version` runs.
- Happy path: on Linux x86_64, same as above.
- Happy path: on Windows x64, `scrapecreators.exe --version` runs.
- Happy path: `npx @scrapecreators/cli tiktok profile charlidamelio --dry-run` runs without a global install.
- Edge case: on an unsupported platform (e.g., linux/riscv64), install fails with a clear error message listing supported platforms.
- Regression: v1's `npx @scrapecreators/cli` examples from its README still work in v2 (command shape unchanged for documented v1 commands).

Verification:
- `npm publish --dry-run` from the npm/ directory produces a well-formed tarball.
- Installing the packed tarball in a clean container on each platform yields a runnable `scrapecreators` binary.

- [ ] U4. Interactive wizard on bare invocation

Goal: Running `scrapecreators` with no args launches a guided wizard (platform -> action -> required params) matching v1's @clack/prompts UX. `--no-input`, `--agent`, or any subcommand disables it.

Requirements: R6.

Dependencies: U2 (action map stable so the wizard can enumerate choices).

Files:
- Create: `library/developer-tools/scrape-creators/internal/cli/wizard.go`
- Modify: `library/developer-tools/scrape-creators/internal/cli/root.go` to detect bare invocation in a TTY with no `--no-input`/`--agent`, and call the wizard instead of printing help
- Create: `library/developer-tools/scrape-creators/internal/cli/wizard_test.go`

Approach:
- Use `github.com/charmbracelet/huh` or `github.com/AlecAivazis/survey/v2` for the prompt library. Match v1's three-step flow: platform picker -> action picker filtered by chosen platform -> param prompts for required fields. Optional params shown only with `--help-style` toggle.
- On completion, render the equivalent CLI invocation so the user can copy it next time. v1 does not do this; PP should. This is the small UX delta that justifies the Go rewrite for humans too.
- Detect non-TTY via `term.IsTerminal(int(os.Stdin.Fd()))`. In non-TTY, bare invocation still prints help (current behavior).
- Respect `--no-input`, `--agent`, and `--yes` flags even when invoked bare: any of them disables the wizard.

Patterns to follow:
- cobra's `Run:` on root command (currently nil; wizard trigger goes here).
- v1's `src/interactive.js` for the three-step structure.

Test scenarios:
- Happy path: in a TTY, `scrapecreators` launches the wizard, the user picks `tiktok profile charlidamelio`, and the request resolves.
- Happy path: the wizard prints the equivalent CLI invocation at the end for copy-paste reuse.
- Edge case: `scrapecreators < /dev/null` (stdin not a TTY) falls back to help output instead of wizard.
- Edge case: `scrapecreators --no-input` falls back to help.
- Edge case: `scrapecreators --agent` falls back to help.
- Error path: user cancels the wizard mid-prompt (Ctrl+C) and exits with code 2.

Verification:
- Bare invocation in a TTY reaches the wizard code path.
- Bare invocation with `--no-input` or non-TTY stdin reaches the help path.

- [ ] U5. `agent add` auto-wiring command

Goal: `scrapecreators agent add cursor|claude|claude-desktop|codex` writes MCP server configuration for the named agent, with a flag to pick hosted MCP (`api.scrapecreators.com/mcp`) or the local `scrapecreators-mcp` binary.

Requirements: R5.

Dependencies: U1.

Files:
- Create: `library/developer-tools/scrape-creators/internal/cli/agent.go`
- Create: `library/developer-tools/scrape-creators/internal/cli/agent_test.go`
- Modify: `library/developer-tools/scrape-creators/internal/cli/root.go` to register the agent subcommand
- Modify: `library/developer-tools/scrape-creators/README.md` (replace the manual JSON block with `scrapecreators agent add <target>` examples)

Approach:
- One Go file per target's config file schema: Cursor (`~/.cursor/mcp.json`), Claude Desktop (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows), Codex (`~/.codex/config.toml`), Claude Code (via `claude mcp add` shell-out, the recommended path per v1's docs).
- Default to local MCP. Add `--hosted` flag to write the hosted MCP URL with `x-api-key` header instead. Mirrors v1's `scrapecreators agent add` UX.
- Never overwrite an existing `scrapecreators` entry without `--force`. Print a diff of what would change and exit cleanly if declined.

Patterns to follow:
- v1's `agent add` command as a UX reference (source not inspected in detail, but README documents `cursor`, `claude`, `codex` as supported targets).
- OS-specific config-path resolution using `os.UserConfigDir()` with platform fallbacks.

Test scenarios:
- Happy path: `scrapecreators agent add claude-desktop` writes a `scrapecreators` entry under `mcpServers` in the correct platform config path.
- Happy path: `scrapecreators agent add cursor --hosted` writes the hosted-MCP URL with the user's stored API key.
- Edge case: target config file does not exist. Command creates it with a minimal schema.
- Edge case: existing `scrapecreators` entry. Command refuses without `--force` and prints a diff.
- Error path: unknown target. Prints a list of supported targets and exits with code 2.
- Error path: no stored API key and `--hosted` picked. Prompts (or exits with code 4 under `--no-input`).

Verification:
- Each target's config file validates against its known schema after `agent add`.

- [ ] U6. MCP parity (hosted + local)

Goal: One MCP artifact, two invocation modes. `scrapecreators-mcp` runs locally by default (stdio). An HTTP proxy mode relays requests to `api.scrapecreators.com/mcp` with the user's API key, so agents that only speak HTTP MCP can use the hosted version too.

Requirements: R5.

Dependencies: U1.

Files:
- Modify: `library/developer-tools/scrape-creators/cmd/scrapecreators-mcp/main.go`
- Modify: `library/developer-tools/scrape-creators/internal/mcp/` (add hosted proxy mode)
- Create: `library/developer-tools/scrape-creators/internal/mcp/hosted_proxy_test.go`

Approach:
- Default behavior unchanged: local stdio MCP server that implements the same tools the cobra tree exposes.
- Add `--mode hosted|local` flag (default local). In hosted mode, proxy incoming MCP JSON-RPC to the hosted endpoint with `x-api-key` from config.
- The auto-wiring command (U5) picks which mode to write into the target agent's config.

Patterns to follow:
- Existing `internal/mcp/` tool manifest pattern.
- MCP JSON-RPC proxy patterns from reference implementations.

Test scenarios:
- Happy path: local mode responds to `initialize`, `tools/list`, `tools/call` per MCP spec.
- Happy path: hosted mode forwards a `tools/call` to `api.scrapecreators.com/mcp` and returns the response.
- Edge case: hosted mode with a missing API key exits with code 4 and a clear error.
- Error path: hosted endpoint returns 429. Proxy returns an MCP error with `retry_after` in the error data.
- Integration: agent-add then tool-call round-trip through both modes.

Verification:
- Both modes pass an MCP smoke test asserting tool list and one tool call.

- [ ] U7. Test coverage for security and command-tree regression

Goal: Port v1's `security.test.js` assertions to Go, plus a generated suite that walks every registered cobra command and asserts basic correctness.

Requirements: R11.

Dependencies: U2.

Files:
- Create: `library/developer-tools/scrape-creators/internal/security/validate_test.go` (URL sanitization, API key redaction from logs and errors, command injection, path traversal)
- Create: `library/developer-tools/scrape-creators/internal/cli/commands_regression_test.go` (walks cobra tree, asserts every `platform action` has `--help`, produces valid JSON on `--dry-run`, rejects unknown `--select` fields)

Approach:
- Read v1's `test/security.test.js` (13,606 bytes) and port each assertion one-to-one. v1 covers URL validation (handles without `@`, hashtags without `#`, allowed domains), API key redaction from error messages and logs, resistance to path traversal in `--output`, and command-injection resistance in interactive input. These are the exact failure modes v2 inherits.
- The regression test uses `cobra.Command.VisitCommands` to enumerate the tree and assert invariants. This catches drift in `action_map.yaml` over time.

Patterns to follow:
- Go `testing` + table-driven tests.
- cobra's `Command.Help` and `Command.Execute` for test invocation.

Test scenarios:
- Every security assertion in v1's `security.test.js` has an equivalent Go test with the same input and expected outcome.
- Every `platform action` pair responds to `--help` with non-empty output.
- Every `platform action` pair accepts `--dry-run` and emits valid JSON describing the would-be request.
- `--select` with an unknown field returns exit code 2 and a helpful error.
- Injection: `--output` cannot write outside the configured working directory without an absolute path.
- Redaction: API key does not appear in any error message written to stderr.

Verification:
- `go test ./...` passes locally.
- Verify-skills CI passes against the renamed binary.

- [ ] U8. README restructure and migration guide

Goal: README leads with Adrian's audience (Node users, prospective v1 migrators) and documents the v1 -> v2 migration. Input normalization, exit codes, and Adrian-style examples are at the top.

Requirements: R9, R10.

Dependencies: U2, U3, U5.

Files:
- Modify: `library/developer-tools/scrape-creators/README.md`
- Create: `library/developer-tools/scrape-creators/MIGRATION.md` (v1 -> v2 command map, flag aliases, breaking changes)
- Modify: `library/developer-tools/scrape-creators/SKILL.md` if present (regenerate)

Approach:
- Restructure the README in this order: tagline, install (npm first, then go install, then homebrew, then binary), quick-start (API key, first command, interactive mode, agent add), input normalization block lifted from v1 verbatim, output formats, unique-to-v2 capabilities (sync, analytics, superpowers) with short example each, MCP (with `agent add` one-liner), troubleshooting with exit codes.
- Migration guide is a table: v1 command -> v2 command, v1 flag -> v2 flag, anything behavioral that changed. Short, table-dominant.

Patterns to follow:
- v1's README order (install / auth / command / discovery / formats / agents / troubleshooting). Lift the structure; repopulate the content with v2 equivalents.

Test scenarios:
- Every code sample in the README is runnable against a test environment with a stub API key.
- Every migration table entry is tested by U7's regression suite (v1 command -> v2 command resolves to same endpoint).
- Link check: no broken internal anchors, and homepage link goes to `scrapecreators.com`.

Verification:
- README renders on GitHub with no broken headings or tables.
- MIGRATION.md covers every command in v1's `command-registry.js`.

## System-Wide Impact

- Interaction graph: the renamed binary and MCP pair interact with cobra command registration, the Printing Press skill generator (`tools/generate-skills`), the verify-skills CI, and any Claude Code plugin users who install `pp-scrape-creators`. The rename ripples into `skills/pp-scrape-creators/SKILL.md` and `registry.json`.
- Error propagation: exit codes `0/2/3/4/5/7/10` must stay stable once documented. Future endpoint additions should keep the same taxonomy.
- State lifecycle risks: local SQLite DB location is user-visible. Migrating the config file path from `scrape-creators-pp-cli` to `scrapecreators` needs a one-time copy with a warning, not a silent delete.
- API surface parity: v1's flag surface is a subset. v2 must accept every v1 flag as an alias, at minimum for the endpoints v1 exposed. Breaking flag behavior is a migration failure.
- Integration coverage: the `agent add` command writes external config files. Tests need to create and verify each target's config in a temp directory; do not touch the real user config during CI.
- Unchanged invariants: the underlying API URL, API key header, and response JSON shape are all unchanged. v2 does not reshape responses. `--select` post-filters; it does not alter the server contract.

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Adrian does not want to adopt the PP build. | Plan is neutral on who owns publishing. Closing these blockers makes the PP build adoption-ready regardless of Adrian's decision. |
| Command rename breaks existing PP users. | Operation-ID names remain as hidden aliases for at least one major version. MIGRATION.md documents the move. |
| npm publish requires `@scrapecreators` scope owned by Adrian. | U3 produces the tarball; publishing is deferred to Follow-Up Work (Adrian's action). Local `npm pack` + `npm i -g ./tarball` works for validation without the scope. |
| Config path migration corrupts existing data. | First-run migration is copy-then-warn, never delete. Keep v1's path readable for one major version. |
| Renaming the binary breaks the Printing Press convention. | Mark the exception in `.printing-press.json` and `AGENTS.md` so the generator does not re-add the `-pp-` infix on regeneration. |
| Hosted MCP endpoint changes shape. | U6's hosted mode is a thin proxy, not a re-implementation. If the hosted endpoint moves, update one URL constant. |
| Test port from `security.test.js` misses a v1 assertion. | U7 is paranoid-by-default: port one-to-one, fail closed if an assertion cannot be mapped. |
| Wizard choice-set drifts from `action_map.yaml`. | Wizard reads action_map at runtime; U7's regression suite asserts the two agree. |

## Documentation / Operational Notes

- Bump `.claude-plugin/plugin.json` version on every change that touches `skills/pp-scrape-creators/SKILL.md` (per AGENTS.md rule).
- Update `registry.json` entry with the new `cli_name`, `binary_name`, and any new aliases.
- Add a CHANGELOG entry under `library/developer-tools/scrape-creators/` documenting the v1 -> v2 migration.
- Coordinate with Adrian (`adrianhorning08` on GitHub, ScrapeCreators org) before publishing to npm. The plan's output is a publishable tarball and a migration guide; the timing and authorship of the actual npm publish is his call.

## Sources & References

- PR under review: `https://github.com/mvanhorn/printing-press-library/pull/113`
- v1 source: `github.com/ScrapeCreators/scrapecreators-cli`
- v1 npm: `https://www.npmjs.com/package/@scrapecreators/cli`
- v1 key files: `src/command-registry.js` (operation-ID -> platform/action dedup), `src/interactive.js` (wizard), `bin/scrapecreators.js` (entry), `test/security.test.js` (security assertions)
- PP README already cites Adrian's prior art: `library/developer-tools/scrape-creators/README.md` Sources & Inspiration section (`adrianhorning08/n8n-nodes-scrape-creators`, `adrianhorning08/scrape-creators-examples`)
- Related PP plan: `docs/plans/2026-04-19-001-feat-ppl-brew-of-clis-and-distribution-plan.md` (npm platform-dispatch pattern)
- Printing Press library conventions: `AGENTS.md` (binary naming, SKILL.md sync, `.printing-press.json` identity)
