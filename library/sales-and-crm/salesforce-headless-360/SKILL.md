---
name: pp-salesforce-headless-360
description: "The agent-context packager for Salesforce. Signed, FLS-safe, cross-surface bundles any agent can consume. Trigger phrases: `customer context for salesforce`, `bundle salesforce account`, `salesforce meeting prep`, `verify salesforce bundle`, `freshness score salesforce`, `use salesforce-headless-360`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["salesforce-headless-360-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/salesforce-headless-360-pp-cli/...@latest","bins":["salesforce-headless-360-pp-cli"],"label":"Install via go install"}]}}'
---

# Salesforce Headless 360

Use this skill when an agent needs portable Salesforce Customer 360 context, signed bundle verification, account freshness scoring, or Slack injection with Salesforce audience safety.

## Killer Commands

```bash
# Build one signed Customer 360 bundle for an agent.
salesforce-headless-360-pp-cli agent context 001xx000003DGb2AAG --since P90D --output acme.json

# Verify before an agent trusts or acts on a bundle.
salesforce-headless-360-pp-cli agent verify acme.json --strict

# Post a field-gated summary into Slack after channel-audience FLS intersection.
salesforce-headless-360-pp-cli agent inject --slack C123456 --bundle acme.json
```

## Safety Notes

- FLS enforcement is always on before Salesforce records enter bundles, sync output, Data Cloud enrichment, or Slack linkage summaries.
- JWT auth requires `agent context --run-as-user <UserId>` for bundle emission; integration-user permissions are not treated as the human user's FLS boundary.
- Slack inject re-FLSes the bundle against the Slack channel audience and blocks unmapped or external members unless the caller explicitly waives that guard.
- `doctor` is local and has no telemetry; use `doctor --mock` as the first smoke test when auth or infrastructure is uncertain.
- If doctor detects Agentforce MCP or Salesforce DX MCP, prefer those tools for tasks they cover directly.

## When To Use Each Verb

Use `context` for broad cross-surface Customer 360 packaging: Account, related CRM records, files, optional Data Cloud profile, and linked Slack context in one signed artifact.

Use `brief` for a narrow one-opportunity handoff when a human or agent needs deal context without a full account bundle.

Use `decay` for freshness triage. It returns a score and signal breakdown that agents can sort or branch on.

Use `verify` before trusting a bundle. Add `--strict` when the next step can mutate systems or expose data.

Use `inject` only when the target Slack audience is the intended audience. It is for collaboration handoff, not bulk publishing.

## Install

```bash
go install github.com/mvanhorn/salesforce-headless-360-pp-cli/...@latest
salesforce-headless-360-pp-cli doctor --mock
```

## Authentication

```bash
salesforce-headless-360-pp-cli auth login --sf prod
salesforce-headless-360-pp-cli auth login --web --client-id "$SF_CLIENT_ID" --org sandbox
salesforce-headless-360-pp-cli auth login --jwt --org ci
```

Run `salesforce-headless-360-pp-cli doctor` after auth. The doctor rows show REST, Data Cloud, Slack linkage, Slack Web API, trust key store, Apex companion, local store, sf CLI passthrough, and competing-tool status.

## Direct Use

If the user provides arguments, run the CLI with those arguments. Prefer `--agent` for machine-readable output unless the user asks for human output.

```bash
salesforce-headless-360-pp-cli <args> --agent
```
