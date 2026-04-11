---
title: Natural-language cart operations without browser control
type: feat
status: active
date: 2026-04-11
origin: none (direct planning request, no upstream brainstorm)
---

# Natural-language cart operations without browser control

## Overview

Today the Instacart CLI can only do cart mutations when the caller already has an exact `items_<loc>-<prod>` id. Resolving a product from a natural-language query (`"2% milk"`) still requires spinning up `browser-use`, clicking an Add button, and reading the resulting `UpdateCartItemsMutation` variables out of Apollo's mutationStore. That defeats the whole premise of a fast, single-binary CLI and leaks the browser-automation dependency we explicitly set out to avoid.

This plan rewires the `search`, `add`, and `cart show` commands to resolve products end-to-end through direct GraphQL replay against `https://www.instacart.com/graphql` using captured persisted query hashes and the existing Chrome-cookie session. No Playwright. No `browser-use`. No click-to-capture.

The work is non-trivial but bounded: we already have every GraphQL hash we need from the earlier sniff, the cart-show name-resolution chain (`CartData` → `ShopCollectionScoped` → `Items`) was verified live during the last session, and the HTTP + session plumbing in `internal/gql/client.go` already handles persisted queries. The remaining question is which of the captured search hashes is the shortest viable path, and how to bootstrap a fresh `retailerInventorySessionToken` for the retailer-scoped search operations that require one.

## Problem Frame

**Who:** Matt (CLI user) and any AI agent invoking the CLI on his behalf.

**What they hit today:**

- `instacart add "2% milk" costco` falls through to an "natural-language search did not resolve" error because `searchHTML` scrapes the client-rendered HTML shell which contains zero product data.
- `instacart search "2% milk" --store costco` returns "no results" for the same reason.
- `instacart cart show pcc-community-markets` returns an item count only, never actual item names, because it calls `CartItemCount` and nothing else.

**What they want:**

- `instacart add "2% milk" costco` returns `added to costco cart: Kirkland Signature 2% Reduced Fat Milk, 1 gal, 2-count (items_1576-17315429)` after a single sub-second round trip.
- `instacart cart show pcc-community-markets` lists every item in the cart with name, size, quantity, and item id.
- `instacart search "2% milk" --store costco --limit 5` returns a ranked list of real products with ids, names, and prices.

**What blocks us today:** the HTML scraping path is a dead end (the page is a React shell) and the captured GraphQL search hashes need slightly different variables/headers than we've been sending. The `retailerInventorySessionToken` in particular needs a bootstrap path we haven't nailed down.

## Requirements Trace

- **R1.** `instacart add "<query>" <retailer>` resolves an item id via GraphQL and fires `UpdateCartItemsMutation`, with no `browser-use` or `agent-browser` involvement at runtime. Works for at least Costco, PCC Community Markets, and one other retailer the CLI has never seen before.
- **R2.** `instacart search "<query>" --store <retailer>` returns 1-N products (name, item id, product id, price if available) in under 2 seconds cold and under 500ms warm.
- **R3.** `instacart cart show <retailer>` returns each line item's name and quantity, not just an item count. Uses only captured persisted query hashes.
- **R4.** When the `retailerInventorySessionToken` the CLI has cached is stale or missing, the CLI refreshes it transparently on the next call by invoking a bootstrap GraphQL operation. User sees at most one extra round trip, not an error.
- **R5.** When a persisted query hash itself is stale (Instacart rolled a new web bundle), the CLI fails with an actionable message pointing at `instacart capture` and exits with a typed code so agents can detect it.
- **R6.** The hardcoded `--item-id` flag on `instacart add` becomes an *override*, not the primary path. Natural-language resolution is the default. `--item-id` stays for power users and agent short-circuits.
- **R7.** Every new code path has at least happy-path integration coverage and one error-path test.

## Scope Boundaries

In scope:

- `internal/cli/search.go`, `internal/cli/add.go`, `internal/cli/carts.go` rewiring
- New `internal/gql/` helpers for search and cart resolution
- Extending the persisted-op cache schema / seed set with the new hashes
- Local SQLite cache for resolved product names and (where possible) the retailer inventory token

Explicitly out of scope:

- Past order history, delivery windows, or Instacart+ status commands (covered in the follow-up plan referenced in `project_instacart_pp_cli.md` memory)
- `instacart capture --live` browser-assisted hash refresh (the stale-hash path in R5 just prints a message; live refresh is a separate plan)
- Any checkout operation
- A swap command (`cart swap --from a --to b`), even though Matt asked for one inline
- Editing retailers / addresses / payment methods
- Rewriting the GraphQL client. `internal/gql/client.go` stays structurally the same; only its callers and op registry grow.

## Context & Research

### Relevant Code and Patterns

- `internal/cli/search.go:86-215` (current `searchHTML` + `extractSearchResults` HTML scraping implementation — to be deleted)
- `internal/cli/add.go:45-140` (current arg handling where `--item-id` is the primary path; needs to flip)
- `internal/cli/carts.go:67-103` (`cart show` currently uses only `CartItemCount`; needs to chain `CartData` + `Items`)
- `internal/cli/add.go:140-210` (`resolveActiveCartID` — already handles `PersonalActiveCarts` → slug match fallback; good pattern to reuse for retailer-context lookups)
- `internal/gql/client.go:75-210` (`Client.call` — already distinguishes GET queries from POST mutations, handles the persisted query envelope, has the `apollographql-client-name` header set. Retry on `PersistedQueryNotFound` exists but doesn't handle `PersistedQueryNotSupported`)
- `internal/instacart/ops.go` (the current `DefaultOps` map — where new hashes will be registered)
- `internal/store/store.go:60-150` (`UpsertOp`, `LookupOp`, `UpsertRetailer` — the cache primitives new code should use)
- `internal/auth/auth.go:ApplyToRequest` (cookie application — unchanged, but new GraphQL helpers must call it)

### Existing Patterns to Follow

- **Config + store + session handoff:** Every command uses `newAppContext(cmd)`, checks `app.RequireSession()`, and threads `(ctx, session, store, cfg)` through. Match this shape exactly for new helpers.
- **JSON-first output:** Every public command honors the root `--json` flag via `app.JSON`. New commands must too.
- **Typed exit codes:** `ExitUsage=2, ExitAuth=3, ExitNotFound=4, ExitConflict=5, ExitTransient=7`. New code uses `coded(...)` for every error path that leaves a command.
- **Raw-body parsing:** GraphQL responses come back as `resp.RawBody []byte`; each caller unmarshals into a local typed wrapper `struct { Data <OpResponse> }`. Keep that pattern for readability instead of introducing a generics layer.

### Institutional Learnings (from prior session memory)

- The `retailerInventorySessionToken` format `v1.<hash>.<customerId>-<zip>-<stuff>-<retailerId>-<locationId>-<stuff>-<stuff>` embeds server-signed state. Do not try to construct it from known fields; it will fail validation. Fetch a fresh one from a bootstrap operation instead.
- Instacart's server rejects `UpdateCartItemsMutation` POSTs that send `query` text with or without a hash extension, with `PersistedQueryNotSupported`. Mutations must send only the hash. GET queries send the hash in `extensions` URL param. This is already handled in `client.go` but the retry branch only catches `PersistedQueryNotFound` — must be extended to `PersistedQueryNotSupported` for user-facing diagnostics (R5).
- `PersonalActiveCarts` returns `data.userCarts.carts[]`, not `data.personalActiveCarts[]`. The existing fix in `types.go` is correct; new code consuming it should keep using `PersonalActiveCartsResponse.UserCarts.Carts`.
- Matt's context captured during the sniff: postal code <zip>, addressId <address-id>, customer id <id>. These are per-user and live only in `~/.config/instacart/session.json` and (optionally) `config.json`. Do not hardcode them.
- Cookie extraction via `kooky` returns garbage on newer Chrome macOS. `auth import-file` is the current working path. Unchanged by this plan.

### External References

None needed. The entire contract lives on `instacart.com` and we already have the hashes we need from the earlier sniff. No official docs to consult; if we ever need to cross-check, the source of truth is the bundle at `https://www.instacart.com/assets/` which rotates.

## Key Technical Decisions

- **Decision 1: Prefer `CrossRetailerSearchAutosuggestions` as the primary NL search path when it works.** Its sample variables (`{"query":"","limit":10,"retailerIds":[...]}`) do NOT include a `retailerInventorySessionToken`, which eliminates the bootstrap problem entirely. It only works when you already know the retailer's numeric id, but the CLI can get that from `PersonalActiveCarts` (for retailers where a cart exists) or from a one-time `ShopCollectionScoped` call (cached thereafter). *Rationale:* shortest viable chain, no session token bootstrap, no Referer hackery.

- **Decision 2: Fall back to `Autosuggestions` (the retailer-scoped one) when `CrossRetailerSearchAutosuggestions` returns empty or the retailer is not in the active-carts set.** This requires a valid `retailerInventorySessionToken`. *Rationale:* retailer-scoped autosuggest is what the web uses on the storefront, so it is presumed more accurate for ambiguous queries; the token bootstrap cost is amortized by caching.

- **Decision 3: Bootstrap the `retailerInventorySessionToken` from `ShopCollectionScoped`.** This operation was captured with full sample variables (`{"retailerSlug":"costco","postalCode":"<zip>","coordinates":{...},"addressId":"..."}`). Whether it returns the token in its response is unverified — this is the #1 execution-time unknown the implementer must answer with a live probe (see `Open Questions / Deferred to Implementation`). If `ShopCollectionScoped` does not return it, try `StoreSearchLayout` next, then `BiaTabSection`. *Rationale:* `ShopCollectionScoped` is the most natural bootstrap surface because we already call it from `resolveRetailer` in `retailers.go`. Reusing that call amortizes cost.

- **Decision 4: Treat `ViewLayoutSearchResults` as a last-resort fallback.** Its captured sample variables are `{}` (empty object), which means the server probably infers the query from the `Referer` URL (`/store/<slug>/search/<query>`). This is fragile — it depends on an undocumented server behavior and could be removed without warning. Use it only if both autosuggest paths return empty. *Rationale:* we do not want to depend on Referer-path inference as the main path, but it might be the only thing that works for some edge cases, so keep it as a fallback with a clear log line when it fires.

- **Decision 5: Cart-show uses the `CartData` → `ShopCollectionScoped` (for shopId) → `Items` chain.** This exact chain was verified live during the 2026-04-11 session and returned real product names. No autosuggest involved. Cache both retailer→shopId mapping and item_id→name mapping in SQLite to make subsequent `cart show` calls fast. *Rationale:* the chain already works; the only work here is wiring it into Go code and adding a cache layer.

- **Decision 6: `retailerInventorySessionToken` lives in the SQLite store, keyed by retailer slug, with a 24-hour TTL.** Tokens embed the customer id and zip, so if either changes the token is automatically invalidated on next fetch. On every use, check TTL first; on expiry, re-bootstrap. *Rationale:* avoids re-fetching on every call; 24 hours matches the expected user session life and falls well within the token's actual validity window (unverified but observed to be much longer during the sniff).

- **Decision 7: `instacart add` becomes NL-first. `--item-id` stays as an override flag, but the argument shape changes.** Today the command is `instacart add <query> <retailer>` with `--item-id` optional. New shape: `instacart add <retailer> <query...>` with `--item-id` optional. The retailer comes first so the variadic `<query...>` doesn't have to guess where the query ends. *Rationale:* variadic tail args match natural language invocation ("add to costco 2% milk and eggs" maps to `instacart add costco 2% milk`). Swapping the positional order is a breaking change, so also accept the old shape (`<query> <retailer>`) for one release with a deprecation notice detected by "last arg looks like a retailer slug". Flag it in the README changelog.

- **Decision 8: Do NOT touch `internal/gql/client.go` structurally.** Add a new file `internal/gql/search.go` and `internal/gql/cart_items.go` for the new helpers. `client.go` only grows by one branch: map `PersistedQueryNotSupported` (in addition to `PersistedQueryNotFound`) to a typed `ErrHashStale` error so callers can surface it cleanly. *Rationale:* minimizing churn in the transport layer makes the diff small and the risk of regressing cart mutations near zero.

## Open Questions

### Resolved During Planning

- **Q:** Is the HTML scraping in `searchHTML` salvageable? **A:** No. The page is a React shell, zero product data in the initial HTML. Verified by a `curl | grep 'aria-label="Add"'` during the last session. Delete the entire function and its helpers.
- **Q:** Do we need a fresh `retailerInventorySessionToken` at all, or can we reuse the one Matt's browser has? **A:** We cannot read it from Chrome (it lives in Apollo's in-memory cache, not in cookies). We must bootstrap a fresh one via GraphQL. See Decision 3.
- **Q:** Should `cart show` go through the HTML page the way `search` currently does? **A:** No. The HTML shell problem applies equally. Go straight to `CartData` + `Items`.
- **Q:** Should natural-language `add` use LLM-based product disambiguation (pick the "best" match when multiple results come back)? **A:** Not in this plan. Pick the first result. If the user wants a specific one, they can `instacart search` first and pass `--item-id`. Adding an LLM dependency is out of scope.
- **Q:** Do we need a new top-level `instacart capture --live` command to refresh hashes? **A:** Not in this plan. The stale-hash path is an error message pointing at a future command; the actual implementation is deferred.

### Deferred to Implementation — RESOLVED 2026-04-11 by Unit 0 and Unit 1 probes

- **Q1.** ✅ `ShopCollectionScoped` DOES return the `retailerInventorySessionToken` in its response. Sample: `v1.<hash>.<customerId>-<zip>-<signed>-1-<retailerId>-<locationId>-1-0`. The token embeds customerId, zip, retailerId, and retailerLocationId. Probe: `docs/plans/probe-responses/unit1-Q1-ShopCollectionScoped-costco.json`.
- **Q2.** ❌ `CrossRetailerSearchAutosuggestions` requires a `zoneId` variable (not captured in the original sniff sample). With zoneId it returns 10 `AutosuggestCrossRetailerSearchSearchTermAutosuggestion` entries — **text suggestions only, no product item ids**. Not usable as a primary product search path.
- **Q3.** ❌ `ViewLayoutSearchResults` with empty vars + `Referer: /store/costco/search/...` returns HTTP 200 with a valid response but contains **only layout configuration** (`SmartRefinementWeb`, `OnSaleButton`, `resultList.searchResultsGridVariant` etc). **Zero product items** in the response regardless of Referer. This operation is the search-page layout template, not the data source. Drop from the chain entirely.
- **Q4.** ✅ `Autosuggestions` response shape confirmed: `{"data":{"retailerAutosuggestionsV2":[{"__typename":"AutosuggestSearchTermAutosuggestion","id":"...","isNatural":false,"relativeUrl":"/costco/search_v3/milk?tracking.autocomplete_prefix=milk&tracking.autocomplete_term_impression_id=...&tracking.image_thing_id=16902650&tracking.image_thing_type=product&...","retailerSlug":"costco","searchTerm":"milk","queryInstructions":null,"viewSection":{...}}]}}`. **Critical: the `relativeUrl` contains `tracking.image_thing_id=<productId>` query parameters — these are the actual product IDs.** Extract via regex. 10 suggestions returned for query "milk".
- **Q5.** ✅ `autosuggestionSessionId` accepts a freshly-generated UUID per call. No server-side validation against a prior sessionId.
- **Q6.** Cache key: use `item_id` directly. The `items_<locationId>-<productId>` format is globally unique and already embeds location context. One table, no composite keys.
- **Q7. [NEW]** *Does Instacart bake sha256 hashes into their JS bundle at build time, so a CLI could self-refresh hashes by scraping the bundle?* **❌ NO.** Probe fetched 8 webpack bundles (sentry, ahoy, runtime, vendor-react, vendor-ids, plus several chunk bundles) and grep'd for `sha256Hash:"..."` patterns — zero matches. Instacart uses Apollo's runtime hash computation (`sha256(print(documentNode))`) rather than build-time baked hashes. Bundle scrape is not a viable self-refresh path. Unit 10 must pivot to a different strategy (see Unit 10 approach update below).

### Unit 1 Findings: The Working Search Chain

The viable product-resolution chain, verified live on 2026-04-11:

```
1. ShopCollectionScoped(retailerSlug, postalCode, coordinates, addressId)
   → response contains retailerInventorySessionToken
   → token format: v1.<hash>.<customerId>-<zip>-<signed>-<retailerId>-<locationId>-<signed>

2. Autosuggestions(retailerInventorySessionToken, query, autosuggestionSessionId=new_uuid)
   → response: {data: {retailerAutosuggestionsV2: [10 entries]}}
   → each entry has a relativeUrl like
     "/costco/search_v3/milk?...&tracking.image_thing_id=16902650&tracking.image_thing_type=product&..."
   → extract each image_thing_id via regex → list of productIds

3. Items(ids=[items_<locationId>-<productId>, ...], shopId, zoneId, postalCode)
   → response: {data: {items: [{id, name, ...}]}}
   → names are real: "Kirkland Signature Whole Milk, 1 gal", etc.
```

**Required context for step 3:** `locationId` (from the token), `shopId` (from retailer cache or ShopCollectionScoped), `zoneId` (from the token). All three are discoverable from the ShopCollectionScoped response + token parsing, so the CLI only needs the retailer slug as input.

**Important caveats:**
- Autosuggestions returns empty for queries containing `%` or other special characters. The CLI should sanitize: `"2% milk"` → `"2 milk"` or `"milk"` before querying, and let the Items lookup disambiguate by name matching afterward.
- Autosuggestions returns at most 10 entries. For popular queries like "milk", those 10 include the top matches, which is what we want.
- The chain is ~3 round trips (~600ms warm, ~1.5s cold) vs 20-40s for Playwright competitors.

### Unit 10 Approach Update (bundle scrape dead)

Since Q7 resolved negatively, Unit 10 pivots:

- **Primary path: remote hash registry.** Host a `hashes.json` file at `https://raw.githubusercontent.com/mvanhorn/instacart-pp-cli-hashes/main/hashes.json` containing operation-name → hash pairs. `instacart capture` fetches this JSON and upserts into the local store. Maintainer re-sniffs via printing-press once per Instacart bundle rotation and PRs the updated JSON. One maintenance action per rotation serves all CLI users.
- **Fallback path: `instacart capture --browser`.** If the user has `browser-use` installed AND the remote registry is unreachable, shell out to browser-use, walk a minimal flow, extract hashes via `persistedQueryLink.request(mockOp, mockForward)` technique documented in `project_instacart_pp_cli.md` memory. Optional — gracefully no-ops with a helpful message if browser-use is absent.
- **Hard fallback: actionable error.** If both the registry and browser-use fail, the CLI prints `"persisted query hashes are stale. Install browser-use (pip install browser-use) and run 'instacart capture --browser', or wait for the maintainer to update hashes at <registry-url>"`.

The remote registry approach is user-hostile for truly offline use but realistic: Instacart rotates bundles every 1-4 weeks, and one maintainer re-sniffing is cheaper than every CLI user running browser-use.

## High-Level Technical Design

> *This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.*

### NL search resolution chain

```
instacart add costco "2% milk"
        │
        ▼
 cli.add.go: resolve query to item id
        │
        ▼
 gql.search.go: ResolveProduct(retailer, query, limit)
        │
        ├─(A)──▶ CrossRetailerSearchAutosuggestions(retailerIds=[5], query)
        │           │
        │           └── if items with ids → return top N
        │
        ├─(B)──▶ Autosuggestions(token, query, sessionId)
        │           │          ▲
        │           │          └── token from SQLite cache (fresh)
        │           │                or bootstrap via ShopCollectionScoped
        │           │
        │           └── if items with ids → return top N
        │
        └─(C)──▶ ViewLayoutSearchResults(Referer=/store/<slug>/search/<q>)
                    │
                    └── parse result, return top N
                        (fallback only, log the fallback)
```

### Cart-show resolution chain

```
instacart cart show pcc-community-markets
        │
        ▼
 cli.carts.go: find cart id
        │
        ▼
 resolveActiveCartID → PersonalActiveCarts → slug match → cart_id
        │
        ▼
 gql.cart_items.go: FetchCartItems(cart_id)
        │
        ├─▶ CartData(id=cart_id)
        │       │
        │       └── extract [{item_id, qty}] via cartItemCollection.cartItems[]
        │
        ├─▶ ShopCollectionScoped(retailerSlug)
        │       │
        │       └── extract shop_id (cache → retailers table)
        │
        └─▶ Items(ids, shop_id, zone_id, postal_code)
                │
                └── zip with cart items → [{name, qty, price, item_id}]
                    (cache names to products table)
```

### Token bootstrap flow (new, implemented in gql.search.go)

```
getInventoryToken(retailer) {
  t = store.GetInventoryToken(retailer)
  if t != nil && t.expires_at > now() { return t.value }

  // live bootstrap
  resp = Query(ShopCollectionScoped, {retailerSlug, postalCode, coordinates, addressId})
  t = walk resp for a field ending in "SessionToken" or "inventorySessionToken"
  if t == "" { try StoreSearchLayout → BiaTabSection → error }
  store.UpsertInventoryToken(retailer, t, now + 24h)
  return t
}
```

## Implementation Units

- [ ] **Unit 0: Dead-end safety net — re-sniff gate**

**Goal:** Before Unit 1 burns time on probes, verify the captured hash set is still valid at all. If Instacart rolled a web bundle since 2026-04-11, every GET query in this plan will fail with `PersistedQueryNotSupported` and no amount of re-probing will help. This unit catches that case early and routes to a re-sniff instead of a fruitless probe loop.

**Requirements:** Prerequisite for every other unit.

**Dependencies:** Valid session in `~/.config/instacart/session.json`.

**Files:**
- No code changes
- Modify: this plan file (update Unit 0 status and findings)

**Approach:**
- Fire the one already-proven-working query from this session: `CurrentUserFields` via the existing `instacart doctor` command. If doctor still reports `api: ok`, the session + at least one hash is still live.
- Then fire two more captured GET queries that this plan depends on: `PersonalActiveCarts` (already wired, call via `instacart carts`) and one raw `curl` against `CartData` with a known cart id. Both must return 200 with non-empty `data`.
- If any of the three calls returns `PersistedQueryNotSupported`, the hash set is stale and Unit 1 cannot proceed. Halt the plan and switch to the re-sniff fallback: spin up `browser-use`, walk the storefront + search + cart flows one time, extract fresh hashes via Performance API + the `persistedQueryLink.request(mockOp, mockForward)` technique from `project_instacart_pp_cli.md`, update `internal/instacart/ops.go`, then resume Unit 1. Budget: 15 minutes.
- If all three pass, proceed to Unit 1 with confidence that the hash landscape hasn't rotated out from under us.

**Execution note:** Characterization-first. This is a pre-flight check; do not write production code in this unit.

**Patterns to follow:** `instacart doctor` (existing) for the live ping pattern.

**Test scenarios:**
- *Happy path:* all three probes return 200 with valid data → proceed to Unit 1.
- *Error path:* one or more returns `PersistedQueryNotSupported` → halt, re-sniff, resume. Document which hashes were stale in the plan's `Open Questions / Deferred to Implementation` section.
- *Error path:* 401/403 on any probe → session expired, tell the user to run `auth import-file` and retry before proceeding.

**Verification:**
- Three probe results (JSON) in `docs/plans/probe-responses/unit0-<opname>.json`.
- Plan updated with either "hash set confirmed live as of YYYY-MM-DD" or "re-sniff performed, new hashes checked in as commit <sha>".

---

- [ ] **Unit 1: Live probe to resolve the deferred questions**

**Goal:** Answer Q1-Q5 with real HTTP responses so subsequent units can proceed from facts instead of assumptions. Writes no production code; produces a short findings note the rest of the plan consumes.

**Requirements:** Prerequisite for R1, R2, R3, R4.

**Dependencies:** None. Valid Instacart session must already be loaded.

**Files:**
- Modify: `docs/plans/2026-04-11-001-feat-instacart-nl-search-plan.md` (update Deferred to Implementation answers in place)
- No code changes

**Approach:**
- Using `python3` or `curl` with the session cookies from `~/.config/instacart/session.json`, issue these calls and save response bodies under `docs/plans/probe-responses/`:
  1. `ShopCollectionScoped` for `retailerSlug=costco` — grep response for `*SessionToken`, `inventorySessionToken`, or anything that looks like the `v1.<hash>.<...>` shape.
  2. `CrossRetailerSearchAutosuggestions` for `query="2% milk", retailerIds=["5"]` — check if result elements have item ids.
  3. `Autosuggestions` for `query="2% milk"` with whatever token was discovered in step 1 — parse response shape.
  4. `ViewLayoutSearchResults` with `variables={}` and `Referer=https://www.instacart.com/store/costco/search/2%25%20milk` — check if results come back.
  5. Generate a fresh UUID and reuse it as `autosuggestionSessionId` across two back-to-back `Autosuggestions` calls — confirm the server accepts it.
- Update this plan file with the answers. Do not proceed to Unit 2 until Q1-Q3 are definitively answered.

**Execution note:** Characterization-first. This unit explicitly runs live HTTP probes, captures responses verbatim, and commits the findings back into the plan. It is the only unit where "run live code" is the goal.

**Patterns to follow:** none — this is a one-off investigation.

**Test scenarios:**
- *Integration:* All five probes return valid JSON and either confirm or refute each assumption. If any probe returns `PersistedQueryNotSupported`, halt and report — that hash is stale and the whole plan needs a `capture --live` bootstrap first.

**Verification:**
- `docs/plans/probe-responses/` contains five JSON files with non-empty bodies.
- This plan's `Deferred to Implementation` section has answers for Q1-Q5.

---

- [ ] **Unit 2: Extend the store schema and ops registry**

**Goal:** Give the rest of the code a place to cache the `retailerInventorySessionToken`, resolved product names, and the new search-related hashes. No behavior change on its own — this unit is pure plumbing.

**Requirements:** R4 (token caching), R3 (product name cache).

**Dependencies:** Unit 1 must be complete so we know which hashes to seed.

**Files:**
- Modify: `internal/store/store.go` — add `inventory_tokens` table and `products` upserts for name caching (the `products` and `products_fts` tables already exist; just new upsert helpers).
- Modify: `internal/instacart/ops.go` — add hash entries for `CrossRetailerSearchAutosuggestions`, `Autosuggestions`, `ViewLayoutSearchResults`, `CartData`, `Items`, and whichever operation Unit 1 confirmed as the bootstrap source.
- Modify: `internal/cli/capture.go` — nothing structural, just ensure the new ops get seeded by the existing loop over `instacart.OpNames()`.
- Test: `internal/store/store_test.go` (new file)

**Approach:**
- `inventory_tokens` schema: `retailer_slug TEXT PRIMARY KEY, token TEXT NOT NULL, fetched_at INTEGER NOT NULL, expires_at INTEGER NOT NULL`.
- Helpers on `*Store`: `UpsertInventoryToken(retailer, token, ttl)`, `GetInventoryToken(retailer) (*InventoryToken, error)` (returns nil on expiry).
- Product name cache: extend `UpsertProduct` to take `(item_id, name, retailer_slug, shop_id, size, price_cents)`; reuse the existing `products_fts` table for offline lookup.
- Hash registration follows the existing `DefaultOps` map style in `ops.go`. Entries for mutations should leave `Query` empty (mutations go hash-only, matching the existing `UpdateCartItemsMutation` pattern).
- Add `instacart.OpNames()` entries for the new ops so `capture` seeds them.

**Patterns to follow:**
- `store.UpsertOp` and `store.LookupOp` (existing).
- `store.UpsertRetailer` (existing) for the inventory token similar two-step migrate+helper structure.

**Test scenarios:**
- *Happy path:* `UpsertInventoryToken("costco", "v1.abc.<customerId>-<zip>", 24h)` followed by `GetInventoryToken("costco")` returns the token.
- *Edge case:* `GetInventoryToken` after the stored `expires_at` returns `nil, nil` (not an error — expiration is normal).
- *Edge case:* upserting an existing slug overwrites the old token rather than erroring.
- *Happy path:* `UpsertProduct` followed by `FTS search "milk"` returns the product.
- *Error path:* loading an ops map with a seed that has no hash leaves the store empty and does not crash.

**Verification:**
- `go test ./internal/store/...` passes.
- `instacart capture` prints `seeded N GraphQL operations` where N >= 18 (was 13 before this unit).
- `instacart ops list --json` shows the new operation names.

---

- [ ] **Unit 3: GraphQL search helper in `internal/gql/search.go`**

**Goal:** A single `ResolveProducts(ctx, app, retailerSlug, query, limit) ([]SearchResult, error)` function that chains the three-path strategy from Decision 1/2/4. This is the core of the feature.

**Requirements:** R1, R2.

**Dependencies:** Unit 2 (store + ops registry).

**Files:**
- Create: `internal/gql/search.go`
- Modify: `internal/instacart/types.go` — add `Autosuggestions`, `CrossRetailerSearchAutosuggestions`, `ViewLayoutSearchResults` response structs. Actual shapes from Unit 1 probe outputs.
- Create: `internal/gql/search_test.go`

**Approach:**
- `ResolveProducts` tries paths A/B/C in the order defined in Decision 1/2/4 and returns on the first success.
- Path A: `CrossRetailerSearchAutosuggestions` — needs only `retailerIds` (resolved from `store.GetRetailer(slug).RetailerID`). If the retailer is not cached, call `ShopCollectionScoped` first to populate it.
- Path B: `Autosuggestions` — needs `retailerInventorySessionToken`. Use `getInventoryToken(slug)` which hits the cache first and bootstraps from `ShopCollectionScoped` (or whichever op Unit 1 confirmed) on miss.
- Path C: `ViewLayoutSearchResults` — constructs the search URL as Referer, fires with empty vars. Only used as fallback with a log line.
- Every path populates the local `products` cache with whatever it resolves, so follow-up calls get warm data.
- Error handling: if all three paths return empty, return `coded(ExitNotFound, "no results for %q at %s")`. If any path returns `PersistedQueryNotSupported`, short-circuit immediately with `ErrHashStale` so the user sees the right error (R5).

**Execution note:** Test-first. Write failing tests against a table-driven fixture of the five Unit 1 probe responses before implementing the resolver. The probe responses become the fixture inputs.

**Patterns to follow:**
- `internal/cli/add.go:resolveActiveCartID` — same "try method 1, fall back to method 2, return empty on no match" shape.
- `internal/gql/client.go:call` — existing request construction pattern.

**Test scenarios:**
- *Happy path:* path A returns two items, `ResolveProducts` returns them and caches names. Second call for the same query hits the cache and never touches the network (test with a mock transport).
- *Happy path:* path A returns empty, path B returns two items, `ResolveProducts` returns path B's results. One token bootstrap happens.
- *Edge case:* cached inventory token is expired, `getInventoryToken` re-bootstraps, call proceeds.
- *Edge case:* retailer is not cached, `ResolveProducts` transparently calls `ShopCollectionScoped` to populate it, then proceeds.
- *Error path:* all three paths return empty → `ExitNotFound` with the right error text.
- *Error path:* any path returns `PersistedQueryNotSupported` → `ErrHashStale`, caller sees `ExitTransient` with a pointer to `instacart capture`.
- *Integration:* live call against real Instacart for `"2% milk"` at `costco` returns at least one result with a populated `ItemID` starting with `items_`. Skipped when `INSTACART_LIVE_TEST` env is unset.

**Verification:**
- `go test ./internal/gql/...` passes.
- Live integration test (gated by env var) returns a result with a valid item id.

---

- [ ] **Unit 4: GraphQL cart-items helper in `internal/gql/cart_items.go`**

**Goal:** A single `FetchCartItems(ctx, app, cartID, retailerSlug) ([]CartLineItem, error)` function that implements the `CartData` → `ShopCollectionScoped` → `Items` chain and caches resolved names.

**Requirements:** R3.

**Dependencies:** Unit 2 (store).

**Files:**
- Create: `internal/gql/cart_items.go`
- Modify: `internal/instacart/types.go` — add `CartDataResponse`, `ItemsResponse`, `CartLineItem` types.
- Create: `internal/gql/cart_items_test.go`

**Approach:**
- Call `CartData` with `{id: cartID}`. Walk `data.userCart.cartItemCollection.cartItems[]` for each line's `basketProduct.id` (the `items_<loc>-<prod>` item id) and `quantity`.
- Look up shop id: first check the local `retailers` cache, fall back to `ShopCollectionScoped(retailerSlug)` which is called with the user's address context. Cache the result in `retailers`.
- Call `Items` with `{ids, shopId, zoneId, postalCode}`. Zone and postal come from `app.Cfg`.
- Zip the results into `CartLineItem{Name, ItemID, Quantity, QuantityType, PriceDisplay, Size}`.
- Upsert every resolved product into the local `products` table so future `cart show` calls get warm data.

**Patterns to follow:**
- Same request construction as Unit 3.
- `cli/add.go:resolveActiveCartID` style for "check cache, fall back to live".

**Test scenarios:**
- *Happy path:* a two-item PCC cart resolves to two line items with real names (`FIELD DAY Tomato Ketchup, Organic, Classic` and `Organic Strawberries Package`, matching the verified 2026-04-11 session). Test uses a fixture.
- *Happy path:* second call for the same cart id hits the local cache for names and skips the `Items` call entirely.
- *Edge case:* empty cart (`cartItemCollection.cartItems = []`) returns an empty slice, not an error.
- *Error path:* `CartData` returns `PersistedQueryNotSupported` → `ErrHashStale`.
- *Error path:* `shopId` unresolvable → clear error pointing at `instacart retailers show <slug>`.
- *Integration:* live call against Matt's actual PCC cart returns the real items (gated by env var, same as Unit 3).

**Verification:**
- `go test ./internal/gql/...` passes for the new test file.
- Live test returns at least one item with `ItemID` starting with `items_`.

---

- [ ] **Unit 5: Rewire `cli/search.go` to use the new resolver**

**Goal:** Delete the dead HTML scraping path and wire `instacart search <query> --store <slug>` to `gql.ResolveProducts`.

**Requirements:** R2.

**Dependencies:** Unit 3.

**Files:**
- Modify: `internal/cli/search.go` (rewrite)
- Test: `internal/cli/search_test.go` (new)

**Approach:**
- Delete `searchHTML`, `extractSearchResults`, and `extractItemIDs` entirely.
- `newSearchCmd` calls `gql.ResolveProducts(app.Ctx, app, store, query, limit)` and formats output.
- Keep the existing `SearchResult` struct (move it to `internal/gql/search.go` alongside the helper, re-export from the cli package via type alias for backwards compatibility of any test touching it).
- Table output format: `"%2d. %-50s item_id=%-30s %s\n"` showing name, item id, price.
- JSON output: array of `SearchResult` unchanged.

**Patterns to follow:**
- `cli/retailers.go:newRetailersListCmd` — same output style for table mode.

**Test scenarios:**
- *Happy path:* `instacart search "2% milk" --store costco --limit 3` returns three results in table form.
- *Happy path:* with `--json` returns a JSON array of three objects each with `name`, `item_id`, `product_id`, `retailer`.
- *Error path:* no `--store` flag returns `ExitUsage` and a message pointing at the flag.
- *Error path:* unknown retailer returns `ExitNotFound`.
- *Integration:* live call returns real results (gated).

**Verification:**
- `go test ./internal/cli/...` passes.
- `instacart search "2% milk" --store costco --limit 5` returns at least one result with a real name and item id end-to-end.

---

- [ ] **Unit 6: Rewire `cli/add.go` to be NL-first**

**Goal:** Flip `instacart add` so natural-language resolution is the default path, `--item-id` becomes an override, and the argument shape becomes `instacart add <retailer> <query...>` (with a backward-compat detector for the old shape).

**Requirements:** R1, R6.

**Dependencies:** Unit 3.

**Files:**
- Modify: `internal/cli/add.go`
- Test: `internal/cli/add_test.go` (new)

**Approach:**
- Arg shape: `cobra.MinimumNArgs(1)`. If `--item-id` is set, first positional is the retailer. If not, first positional is the retailer and the rest are joined as the query.
- Backward compat: if the last arg matches a known retailer slug (via `store.GetRetailer`) AND `--item-id` is not set AND there are 2+ args, assume the old shape `<query> <retailer>` and print a one-line deprecation notice to stderr: `note: "instacart add <query> <retailer>" is deprecated, use "instacart add <retailer> <query...>"`. This works for one release.
- Replace the current "best-effort NL via searchHTML" branch with `gql.ResolveProducts` (top result wins).
- Keep `--qty`, `--yes`, `--dry-run`, `--cart-id` flags unchanged.
- Resolved product confirmation prompt should print the full name (not the stub `(item id supplied: ...)`) so users know what they're about to add.
- Still delegates the mutation to the existing `client.Mutation("UpdateCartItemsMutation", vars, "")` path — no changes to the mutation side.

**Patterns to follow:**
- Current `cli/add.go` structure for arg parsing, confirm prompt, mutation call.
- Deprecation warning pattern: borrow from `cobra.Command.Deprecated` semantics but emit our own message to stderr.

**Test scenarios:**
- *Happy path:* `instacart add costco "2% milk" --yes --dry-run` prints the resolved name and item id, does not fire a mutation.
- *Happy path:* `instacart add --item-id items_1576-17315429 costco --yes --dry-run` bypasses NL and uses the explicit id.
- *Happy path:* `instacart add costco "2% milk" --json --dry-run` returns structured JSON with the resolved product.
- *Edge case:* new shape `instacart add costco "eggs" "milk"` joins args into query `"eggs milk"` and resolves once.
- *Edge case:* backward-compat shape `instacart add "2% milk" costco` detects `costco` as a known retailer and warns once.
- *Error path:* unknown retailer → `ExitNotFound` with `"retailer %q not found — run \`instacart retailers list\`"`.
- *Error path:* NL resolution returns empty → `ExitNotFound` with `"no match for %q at %s — try \`instacart search\` first or pass --item-id"`.
- *Error path:* user says "n" at the confirm prompt → `ExitConflict` with `"cancelled by user"`.
- *Integration:* live add of a known-good item via the old `--item-id` path still works (regression).

**Verification:**
- `go test ./internal/cli/...` passes.
- `instacart add costco "2% milk" --yes --dry-run` prints the expected product name without touching the cart.
- `instacart add costco "2% milk" --yes` actually adds the item (manual live check, one-time).

---

- [ ] **Unit 7: Rewire `cart show` to return real item names**

**Goal:** `instacart cart show <retailer>` returns a table/JSON of actual line items with names, quantities, and item ids.

**Requirements:** R3.

**Dependencies:** Unit 4.

**Files:**
- Modify: `internal/cli/carts.go` (`newCartShowCmd`)
- Test: `internal/cli/carts_test.go` (new)

**Approach:**
- `newCartShowCmd` calls `resolveActiveCartID` to get the cart id, then `gql.FetchCartItems(ctx, app, cartID, retailer)`.
- Table format: `"  %2d. %-50s qty=%s  item=%s\n"` per line item, with a header showing cart id and total item count.
- JSON format: `{retailer, cart_id, items: [{name, item_id, quantity, quantity_type}], item_count}`.
- Error handling: cart not found returns `ExitNotFound`; hash stale returns `ExitTransient` with the right pointer.

**Patterns to follow:**
- Existing `cli/carts.go:newCartShowCmd` for command structure; just swap the data source.

**Test scenarios:**
- *Happy path:* cart with two items returns both names in table form.
- *Happy path:* `--json` returns an array of two objects with names and quantities.
- *Edge case:* empty cart returns `"0 items"` message and exits 0 (not 4).
- *Error path:* unknown retailer → `ExitNotFound`.
- *Error path:* hash stale → `ExitTransient` with pointer to `instacart capture`.
- *Integration:* live `cart show pcc-community-markets` returns the real items.

**Verification:**
- `go test ./internal/cli/...` passes.
- `instacart cart show pcc-community-markets` returns at least one real item name end-to-end.

---

- [ ] **Unit 8: Handle `PersistedQueryNotSupported` in the GraphQL client**

**Goal:** Teach `internal/gql/client.go` to recognize `PersistedQueryNotSupported` (in addition to `PersistedQueryNotFound`) and surface a typed `ErrHashStale` error so callers can show a consistent actionable message (R5).

**Requirements:** R5.

**Dependencies:** None, but landing this before Units 3/4/5/6/7 means they can consume `ErrHashStale` directly.

**Files:**
- Modify: `internal/gql/client.go` — extend the error-retry branch.
- Test: `internal/gql/client_test.go` (new or extended)

**Approach:**
- Define `var ErrHashStale = errors.New("graphql persisted query hash is stale — run `instacart capture`")` in `internal/gql/client.go` (or an `errors.go` sibling file).
- In `Client.call` after parsing the response, check each error's message case-insensitively for `PersistedQueryNotSupported` and `PersistedQueryNotFound`. Both map to `ErrHashStale`.
- Callers wrap `ErrHashStale` into `coded(ExitTransient, "%v", err)` for user-facing output.
- Do not try to auto-recover. Stale hash requires a `capture --live` refresh that doesn't exist yet; the user-facing message is the contract.

**Patterns to follow:**
- Existing `Client.call` error handling (lines ~185-200 of `client.go`).

**Test scenarios:**
- *Happy path:* a 200 response with a `data` field returns normally, no error.
- *Error path:* response with `errors: [{message: "PersistedQueryNotFound"}]` returns `ErrHashStale`.
- *Error path:* response with `errors: [{message: "PersistedQueryNotSupported", extensions: {code: "PERSISTED_QUERY_NOT_SUPPORTED"}}]` returns `ErrHashStale`.
- *Error path:* response with some other error returns a generic `graphql errors: %s` error, unchanged from today's behavior.
- *Edge case:* HTTP 401/403 still returns the auth-rejected error, not `ErrHashStale`.

**Verification:**
- `go test ./internal/gql/...` passes.
- Grep the codebase: no caller still references the old `PersistedQueryNotFound`-only check.

---

- [ ] **Unit 9: Update README and delete the `auth paste`/`auth import-file` rough edges**

**Goal:** Remove the "natural-language best-effort" caveats from the README, document the new argument shape for `add`, document the new `cart show` output, and note the new graphql-native search path.

**Requirements:** documentation for R1/R2/R3/R6.

**Dependencies:** Units 3-7 done so the docs reflect reality.

**Files:**
- Modify: `README.md`

**Approach:**
- Delete the "natural-language mode is best-effort" warning in the `add` section.
- Replace with: "`instacart add costco 2% milk` resolves via direct GraphQL search and fires the add mutation in a single round trip."
- Update the Cookbook examples to use NL-first shape.
- Add a Troubleshooting entry: "If you see `graphql persisted query hash is stale`, Instacart rolled a new web bundle. Run `instacart capture` to re-seed from the built-in set. If that still fails, `instacart capture --live` (coming soon) will re-sniff fresh hashes from a headed browser."
- Add a Changelog section at the bottom noting the breaking `add` argument swap and the one-release backward-compat window.
- Scope Boundaries section in the README: still no checkout, still no payment methods.

**Test scenarios:** none, it's docs.

**Verification:**
- `grep -i "best-effort" README.md` returns nothing.
- README Cookbook examples all work against the built binary.

---

## System-Wide Impact

- **Interaction graph:** `cli/add.go`, `cli/search.go`, `cli/carts.go` all become callers of the new `internal/gql/search.go` and `internal/gql/cart_items.go`. `internal/gql/client.go` gains one new error type and one new response-code branch. `internal/store/store.go` gains one new table. `internal/instacart/ops.go` gains ~5 new entries. Nothing else is touched.
- **Error propagation:** New `ErrHashStale` from `internal/gql/client.go` bubbles through `search.ResolveProducts` and `cart_items.FetchCartItems`, gets wrapped into `coded(ExitTransient, ...)` by command-layer callers. Existing auth errors (`ExitAuth`) and not-found errors (`ExitNotFound`) are unchanged.
- **State lifecycle risks:** The inventory token cache has a 24h TTL. If the user changes their Instacart default address, the token becomes invalid. The token bootstrap logic re-fetches on every stale-cache hit, so worst case the user sees one extra 200ms round trip after an address change. No orphaned state.
- **API surface parity:** `instacart add` argument order changes. Backward-compat detection in Unit 6 keeps the old shape working for one release. After that, removing the compat code is a two-line follow-up.
- **Integration coverage:** The plan's integration tests (gated by `INSTACART_LIVE_TEST`) exercise the real HTTP chain for each unit. Because there are no unit tests today, every new file ships with at least one happy-path test. The live tests are the only proof that the hash chain actually works against Instacart's server.
- **Unchanged invariants:** The mutation path (`UpdateCartItemsMutation` via hash-only POST) is not touched. `internal/gql/client.go:call` still distinguishes GET from POST based on the method parameter. Cookie handling in `internal/auth/auth.go` is unchanged. The Chrome cookie extraction path (`auth login`) and the JSON-file fallback (`auth import-file`) are untouched.

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| `ShopCollectionScoped` does not return the `retailerInventorySessionToken` in its response, so the bootstrap path in Decision 3 fails. | Unit 1 probe verifies this before any other code is written. If it fails, probe `StoreSearchLayout` and `BiaTabSection` as alternates (both captured); at least one must work because the web client somehow obtains the token. |
| `CrossRetailerSearchAutosuggestions` returns only text suggestions without item ids, making the primary path useless. | Decision 2's fallback to `Autosuggestions` covers this. If both return empty, Decision 4's `ViewLayoutSearchResults` with Referer is the last resort. If all three fail, the plan was wrong about the hash landscape and we need to re-sniff — Unit 1 probe catches this before coding starts. |
| Instacart rotates the `UpdateCartItemsMutation` hash during this work, breaking `add` and blocking the only live integration test we can rely on. | R5 error message already covers user-facing behavior. If this happens during development, pause and manually re-extract the hash using the `persistedQueryLink.request(mockOp, mockForward)` technique documented in `project_instacart_pp_cli.md`. Maximum 10 minutes of interruption. |
| The backward-compat detection in Unit 6 misclassifies a query that happens to end in a retailer slug (e.g., `instacart add "sliced aldi cheese" aldi` — is `aldi` a query or the retailer?). | Accept the ambiguity: always prefer "last arg is retailer if it matches a known slug". If the user really means that query, they can quote it or pass `--item-id`. Document in README. |
| Integration tests against a live Instacart account are not reproducible in CI and could leave test items in the user's cart. | Gate all live tests behind `INSTACART_LIVE_TEST=1` env var. Each live test must add then immediately remove its test item. Never commit a CI workflow that runs live tests. |
| The `retailerInventorySessionToken` turns out to be bound to a session cookie that rotates more often than we expected, invalidating the 24h TTL. | 24h is an initial guess. Lower to 1h if Unit 1 probe reveals short-lived tokens. TTL lives in one place (`Unit 2`), so this is a one-line change. |
| `Items` query requires a `shopId` that varies per-retailer-location and we can't resolve it for a retailer we've never touched. | `ShopCollectionScoped` returns shop info (verified during the last session when we looked up PCC's shopId = 5929). Same operation that bootstraps the inventory token; resolve both in the same call. |

## Documentation / Operational Notes

- README changelog: breaking argument swap on `instacart add`, deprecation for one release.
- Troubleshooting entry: the new `ErrHashStale` error and how to respond to it (currently "run `instacart capture`", eventually "run `instacart capture --live`").
- Memory note update: `project_instacart_pp_cli.md` should get a trailing line after this plan ships, clearing the "wire cart show and search to use the captured hashes" pending item.
- No monitoring or rollout concerns: this is a single-binary CLI the user runs locally. Failure modes are visible in the terminal.

## Sources & References

- **Origin document:** none (direct planning request from Matt)
- **Memory:** `~/.claude/projects/-Users-mvanhorn/memory/project_instacart_pp_cli.md` — current CLI state, captured hashes, PersistedQueryLink extraction technique, pending feature list, Matt's user context
- **Captured GraphQL operations:** `~/printing-press/.runstate/mvanhorn-eb6a05f2/runs/20260411-011250/discovery/all-operations.json` (84 ops total, 18 directly relevant to this plan)
- **Sniff report:** `~/printing-press/.runstate/mvanhorn-eb6a05f2/runs/20260411-011250/discovery/sniff-report.md`
- **Current source tree:** `~/printing-press/library/instacart/`
- **Binary:** `~/printing-press/library/instacart/instacart` (symlinked at `~/.local/bin/instacart`)
