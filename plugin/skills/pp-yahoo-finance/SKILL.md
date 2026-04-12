---
name: pp-yahoo-finance
description: "Printing Press CLI for Yahoo Finance. Quotes, charts, fundamentals, options chains, and a local portfolio/watchlist tracker against Yahoo Finance — no API key, with Chrome-session fallback for rate-limited IPs Trigger phrases: 'install yahoo-finance', 'use yahoo-finance', 'run yahoo-finance', 'Yahoo Finance commands', 'setup yahoo-finance'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["yahoo-finance-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest","bins":["yahoo-finance-pp-cli"],"label":"Install via go install"}]}}'
---

# Yahoo Finance — Printing Press CLI

Quotes, charts, fundamentals, options chains, and a local portfolio/watchlist tracker against Yahoo Finance — no API key, with Chrome-session fallback for rate-limited IPs

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `yahoo-finance-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest
   ```
3. Verify: `yahoo-finance-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add yahoo-finance-pp-mcp -- yahoo-finance-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which yahoo-finance-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `yahoo-finance-pp-cli --help`
3. Match the user query to the best command. Drill into subcommand help if needed: `yahoo-finance-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   yahoo-finance-pp-cli <command> [subcommand] [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes` for structured, token-efficient output.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
