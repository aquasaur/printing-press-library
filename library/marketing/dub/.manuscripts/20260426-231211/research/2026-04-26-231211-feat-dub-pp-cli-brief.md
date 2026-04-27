# Dub CLI Brief

## API Identity
- **Domain:** Modern link management (often described as the open-source Bitly). AGPL-3.0, self-hostable, vendor-maintained.
- **Users:** Marketers running attribution campaigns, growth engineers automating UTM/short-link generation, developers shipping referral programs, partner-program operators.
- **Data profile:** Links + analytics aggregates dominate. Long-tail resources (partners, commissions, customers, bounties) only matter if you've enabled those features. Many small entities (tags, folders, domains).

## Reachability Risk
- **None.** dubinc/dub repo is highly active (23k stars, last push 2026-04-27), no open issues citing 403s, rate-limit blocks, or deprecations. Spec ships nightly via Speakeasy. The OpenAPI doc says `version: 0.0.1` despite being a mature production API — don't trust the version field for change detection.

## Top Workflows
1. **Bulk campaign link creation** — UTM-tagged variants per channel/audience using `POST /links/bulk`. Organize with tags + folders.
2. **Top-link analytics rollup** — `/analytics?groupBy=top_links&interval=30d` + `/events` stream. Identify winners and tail.
3. **Conversion attribution tracking** — server-side `/track/lead` + `/track/sale` tied to customers for revenue attribution. Native Stripe/Shopify integrations.
4. **Partner program ops** — recruit, approve, ban; set commission rates; process bounties and payouts. Large surface area in the API (9 endpoints).
5. **Domain migration + branded QR** — vanity domain registration, status polling, per-link QR generation with logo overlay.

## Table Stakes
- Create / update / delete short links (with custom slugs, password, expiration, geo targeting, device targeting, A/B variants)
- Bulk operations on links (POST/PATCH/DELETE bulk endpoints)
- List + filter links (by domain, tag, folder, search, date range)
- Get link analytics (clicks, leads, sales aggregated by interval / dimension)
- Tag and folder management
- QR code generation
- Domain CRUD + verification

## Codebase Intelligence
- **Spec source (authoritative):** `https://raw.githubusercontent.com/dubinc/dub-ts/main/.speakeasy/out.openapi.yaml` — 370 KB, OpenAPI 3.0.3, 39 paths / 53 operations. Generated nightly by Speakeasy from internal source. **Do not** point at `api.dub.co/openapi.json` — it 404s.
- **Auth:** Single security scheme `token`, type `http`, scheme `bearer`, env-var hint `DUB_API_KEY`. Header: `Authorization: Bearer $DUB_API_KEY`.
- **Base URL:** `https://api.dub.co`
- **Resource groups (op count):** partners 9, links 6, domains 4, track 3, commissions 3, bounties 3, tags 2, folders 2, customers 2, analytics 1, events 1, payouts 1, embed-tokens 1, qr 1.
- **Verb quirk:** `/links/upsert` and `/partners/links/upsert` are **PUT** (not POST). Generator must handle correctly.
- **Rate limits:** Free 60/min → Enterprise 3000/min, with stricter per-second caps on analytics endpoints (Pro 2/s, Advanced 8/s). Headers: `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After` on 429s.
- **Architecture:** Workspace-scoped API keys (workspace is implicit in the key). No `workspaces` resource in the public spec — created via the dashboard.

## User Vision
- User explicitly said the previous v1.0.1 print is now stale (current is v2.3.9 — major version delta with transcendence/store/novel-feature support added). Regenerating from scratch on the current binary.
- User authorized full dogfood including writes/deletes/mutations. API key provided. No need to be timid in Phase 5 — exercise the live API including create/update/delete cycles.

## Product Thesis
- **Name:** `dub-pp-cli` (Printing Press CLI for Dub)
- **Why it should exist:**
  - **No official CLI exists** — open issue [#506](https://github.com/dubinc/dub/issues/506) has been requesting one. The only community CLI (sujjeee/dubco) is 18 months stale and link-shortening only.
  - The single MCP server (gitmaxd/dubco-mcp-server-npm) wraps **4 of 53 operations**. We can absorb everything in the spec and add features it never had.
  - Power-user marketing workflows (campaign-wide link creation, top-link rollups across windows, partner program ops) are batch-shaped — exactly where a CLI wins over a web UI.
  - Local SQLite store unlocks transcendence: cross-link analytics joins, partner commission audit trails, dead-link detection, performance drift alerts, bulk reorganization with diff/dry-run safety.

## Build Priorities
1. **Foundation:** Local SQLite store for `links`, `tags`, `folders`, `domains`, `analytics_buckets`. Cursor-based sync.
2. **Absorb every operation:** All 53 spec operations as commands with `--json`, `--dry-run`, `--select`, agent-native exit codes.
3. **Transcendence (must-have, not nice-to-have):** dead-link detection, top-performer rollups, drift detection, bulk slug-collision linter, partner commission audit, link health dashboard.
4. **Polish:** Bulk operations safe-by-default (require `--yes` for mutations on >N items), QR generation with stdout-friendly base64/file output, doctor checks workspace key + rate limit headers.
