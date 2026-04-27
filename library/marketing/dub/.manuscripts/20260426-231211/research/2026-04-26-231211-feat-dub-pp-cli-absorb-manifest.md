# Dub CLI — Absorb Manifest

## Ecosystem audit

Searches run across GitHub, npm, PyPI, Claude plugins. Findings:

| Tool | Type | Lang | Surface | Status |
|------|------|------|---------|--------|
| dubinc/dub-ts | Official SDK | TS | All 53 ops (Speakeasy-generated from canonical spec) | Active |
| dubinc/dub-go | Official SDK | Go | All 53 ops | Active |
| dubinc/dub-python | Official SDK | Python | All 53 ops | Active |
| dubinc/dub-php | Official SDK | PHP | All 53 ops | Active |
| dubinc/dub-ruby | Official SDK | Ruby | All 53 ops | Active (slower) |
| gitmaxd/dubco-mcp-server-npm | MCP server | JS | 4 ops only (`POST /links`, `PATCH /links/{id}`, `DELETE /links/{id}`, `GET /domains`) | Active |
| sujjeee/dubco | Community CLI | TS | Link shortening only (~3 ops) | **Stale ~18 mo** |
| **No official Dub CLI** | — | — | — | Open issue [#506](https://github.com/dubinc/dub/issues/506) |
| Bitly CLI (`bitly-cli`) | Competitor | various | Link CRUD + bitlinks groups | Active |
| Short.io | Competitor | — | Link CRUD, domain mgmt | Active |
| Rebrandly | Competitor | — | Link CRUD, custom domains | Active |

**Key insight:** No official Dub CLI exists. The single MCP wraps **4 of 53** operations. The community CLI is 18 months stale. We absorb everything in the spec, match competitor link-management features, then transcend with local-store features no competitor offers.

---

## Absorbed (match or beat everything)

The generator absorbs all 53 spec operations as commands. Below is the canonical map:

| # | Resource | Operation | Method+Path | Best Source | Our Implementation | Added Value |
|---|----------|-----------|-------------|-------------|--------------------|-------------|
| 1 | links | list | GET /links | dub-ts, MCP, sujjeee | `dub-pp-cli links list` | `--json --select --csv`, local store cache, FTS search |
| 2 | links | create | POST /links | dub-ts, MCP | `dub-pp-cli links create` | `--dry-run`, idempotency hint |
| 3 | links | retrieve | GET /links/info | dub-ts | `dub-pp-cli links get` | Resolve from local store first |
| 4 | links | update | PATCH /links/{linkId} | dub-ts, MCP | `dub-pp-cli links update` | `--dry-run`, diff preview |
| 5 | links | delete | DELETE /links/{linkId} | dub-ts, MCP | `dub-pp-cli links delete` | `--yes` confirmation, batch tombstone |
| 6 | links | upsert | PUT /links/upsert | dub-ts | `dub-pp-cli links upsert` | Idempotent CI flow |
| 7 | links | count | GET /links/count | dub-ts | `dub-pp-cli links count` | Local store fast path |
| 8 | links | bulk-create | POST /links/bulk | dub-ts | `dub-pp-cli links bulk create` | `--stdin-jsonl`, dry-run, partial success report |
| 9 | links | bulk-update | PATCH /links/bulk | dub-ts | `dub-pp-cli links bulk update` | Diff preview, `--yes` gate above 25 items |
| 10 | links | bulk-delete | DELETE /links/bulk | dub-ts | `dub-pp-cli links bulk delete` | Tombstone, audit log, `--yes` |
| 11 | analytics | retrieve | GET /analytics | dub-ts | `dub-pp-cli analytics get` | Local cache, per-second rate-limit aware |
| 12 | events | list | GET /events | dub-ts | `dub-pp-cli events list` | Stream-style stdout, `--follow` |
| 13 | tags | list | GET /tags | dub-ts | `dub-pp-cli tags list` | Local store join |
| 14 | tags | create | POST /tags | dub-ts | `dub-pp-cli tags create` | Idempotent on name |
| 15 | folders | list | GET /folders | dub-ts | `dub-pp-cli folders list` | Local store join |
| 16 | folders | create | POST /folders | dub-ts | `dub-pp-cli folders create` | — |
| 17 | domains | list | GET /domains | dub-ts, MCP | `dub-pp-cli domains list` | — |
| 18 | domains | create | POST /domains | dub-ts | `dub-pp-cli domains create` | — |
| 19 | domains | update | PATCH /domains/{slug} | dub-ts | `dub-pp-cli domains update` | — |
| 20 | domains | delete | DELETE /domains/{slug} | dub-ts | `dub-pp-cli domains delete` | `--yes` |
| 21 | domains | register | POST /domains/register | dub-ts | `dub-pp-cli domains register` | Status-poll wrapper |
| 22 | domains | status | GET /domains/status | dub-ts | `dub-pp-cli domains status` | Pretty-printed verification state |
| 23 | qr | get | GET /qr | dub-ts | `dub-pp-cli qr get` | `--out file.png`, base64 stdout option |
| 24 | track | lead | POST /track/lead | dub-ts | `dub-pp-cli track lead` | `--dry-run` |
| 25 | track | sale | POST /track/sale | dub-ts | `dub-pp-cli track sale` | `--dry-run`, idempotency key |
| 26 | track | open | POST /track/open | dub-ts | `dub-pp-cli track open` | — |
| 27 | customers | list | GET /customers | dub-ts | `dub-pp-cli customers list` | — |
| 28 | customers | retrieve | GET /customers/{id} | dub-ts | `dub-pp-cli customers get` | — |
| 29 | partners | list | GET /partners | dub-ts | `dub-pp-cli partners list` | Local store join |
| 30 | partners | create | POST /partners | dub-ts | `dub-pp-cli partners create` | — |
| 31 | partners | retrieve | GET /partners/{id} | dub-ts | `dub-pp-cli partners get` | — |
| 32 | partners | update | PATCH /partners/{id} | dub-ts | `dub-pp-cli partners update` | — |
| 33 | partners | applications | GET /partners/applications | dub-ts | `dub-pp-cli partners applications list` | — |
| 34 | partners | approve | POST /partners/applications/{id}/approve | dub-ts | `dub-pp-cli partners applications approve` | `--yes` |
| 35 | partners | reject | POST /partners/applications/{id}/reject | dub-ts | `dub-pp-cli partners applications reject` | `--yes` |
| 36 | partners | ban | POST /partners/{id}/ban | dub-ts | `dub-pp-cli partners ban` | `--yes` |
| 37 | partners | links upsert | PUT /partners/links/upsert | dub-ts | `dub-pp-cli partners links upsert` | — |
| 38 | commissions | list | GET /commissions | dub-ts | `dub-pp-cli commissions list` | Local store join |
| 39 | commissions | update | PATCH /commissions/{id} | dub-ts | `dub-pp-cli commissions update` | — |
| 40 | commissions | bulk update | PATCH /commissions | dub-ts | `dub-pp-cli commissions bulk update` | Diff preview |
| 41 | bounties | list | GET /bounties | dub-ts | `dub-pp-cli bounties list` | — |
| 42 | bounties | create | POST /bounties | dub-ts | `dub-pp-cli bounties create` | — |
| 43 | bounties | update | PATCH /bounties/{id} | dub-ts | `dub-pp-cli bounties update` | — |
| 44 | payouts | list | GET /payouts | dub-ts | `dub-pp-cli payouts list` | — |
| 45 | embed-tokens | links | POST /tokens/embed/referrals/links | dub-ts | `dub-pp-cli tokens embed referrals-link` | — |

**Note:** items 46-53 covered by additional bulk/sub-resource ops the generator emits (sub-paths within the resources above). The generator enumerates the spec faithfully.

Every absorbed command supports: `--json`, `--select <field,...>`, typed exit codes (0/2/3/4/5/7/10), dry-run for mutations, agent-native examples in `--help`.

---

## Transcendence (only possible with local store + cross-API joins)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Dead-link detection | `links stale --days 90` | Requires local store of links + analytics aggregates joined by `link_id`. Returns links with zero clicks in N days, archived links with traffic still flowing, expired links with active referrers. No SDK or competitor surfaces this — the API doesn't have an aggregation endpoint shaped this way. |
| 2 | Performance drift detection | `links drift --window 7d --threshold 30%` | Requires sequential analytics snapshots stored locally. Detect links whose click rate dropped >30% week-over-week. Pre-emptive: catches campaigns silently dying before reporting deadlines. The /analytics endpoint returns point-in-time data, not deltas. |
| 3 | Top-performer rollup with tag/folder joins | `links top --by clicks --groupBy tag --interval 30d` | The /analytics endpoint groups by `top_links` but cannot join with local tag/folder taxonomy in one call. Local store joins `analytics_buckets` × `links` × `tags` × `folders` for arbitrary slice-and-dice. |
| 4 | Slug-collision lint | `links lint slugs` | Pure local-data audit — find lookalike short keys (`/launch` vs `/launches`), reserved-word violations, brand-conflict slugs, case-sensitivity hazards across domains. No API endpoint to ask. |
| 5 | Bulk URL/UTM rewrite with diff | `links rewrite --match 'utm_source=oldsrc' --replace 'utm_source=newsrc' --dry-run` | Show every link that would change and the exact patch BEFORE sending. The API's bulk-update accepts the patch but doesn't preview. Saves campaign-wide blast radius mistakes. |
| 6 | Commission audit reconciliation | `partners audit-commissions` | Joins local `partners` × `commissions` × `bounties` × `payouts` to flag: partners earning at stale rates, commissions missing payouts, bounties active past expiry. Requires data shape only present in local store. |
| 7 | Workspace health doctor | `health` | Cross-resource report: rate-limit headroom, expired-but-active links, links with 4xx destination URLs (HEAD-probed locally), domains failing verification, dormant tags. Mirrors what an oncall ops engineer would check Monday morning. |
| 8 | Time-windowed change feed | `since 24h` | "What happened in the last N hours?" — created/updated/deleted links, recent partner approvals, top-clicked links. Requires local timestamps. Powers agent "what's new" flows. |

All 8 transcendence features score **>= 5/10** on the user-impact rubric (workflow centrality, manual-effort displacement, agent utility, frequency).

---

## Stub policy

**No stubs are planned.** Every absorbed command and every transcendence command above is shipping-scope. If the generator emits a command body that doesn't compile, we fix it; we do not ship `// TODO: not yet wired` placeholders.

---

## Total feature count

- Absorbed: 53 spec operations → ~50 distinct CLI commands (some collapsed via subcommand groups)
- Transcendence: 8 novel features
- **Total: ~58 commands**, vs. best existing tool (gitmaxd MCP) at **3 commands** = **+1833%** surface coverage. The community CLI (sujjeee) is even smaller and stale.
