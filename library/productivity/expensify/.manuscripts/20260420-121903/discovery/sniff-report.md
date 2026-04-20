# Expensify Sniff Report

## User Goal Flow
- **Goal:** File an expense report from Claude Code (submit expenses, create report, send for approval)
- **Steps completed:**
  1. User logged in to new.expensify.com in headed agent-browser
  2. Navigated to inbox (`/`)
  3. Navigated to expenses search (`/search/expenses`)
  4. Navigated to create-expense route (`/iou/new`)
  5. Navigated to workspaces settings (`/settings/workspaces`)
  6. Clicked chat thread to inspect report details
- **Steps skipped:**
  - Actual expense creation (requires filling a form with real data; risk of creating test clutter in live workspace)
  - Actual report submission (same reason)
- **Coverage:** 5 of 6 planned navigation flows completed

## Pages & Interactions
| # | URL | Purpose | Key actions |
|---|-----|---------|-------------|
| 1 | `https://new.expensify.com/` | Inbox / main chat list | Snapshot, click chat thread |
| 2 | `https://new.expensify.com/search/expenses` | Expense search | Navigate, scroll |
| 3 | `https://new.expensify.com/iou/new` | Create expense route | Navigate |
| 4 | `https://new.expensify.com/search/expenses?status=all` | All-status expense search | Navigate |
| 5 | `https://new.expensify.com/settings/workspaces` | Workspace settings | Navigate |

## Sniff Configuration
- Backend: agent-browser 0.23.4 (headed, --session expensify-auth)
- Pacing: 3s between navigations (single-user session, no rate limit risk)
- Proxy pattern: NOT detected — no proxy envelope. API calls target `https://www.expensify.com/api/<CommandName>`.

## Endpoints Discovered

**API shape:** `POST https://www.expensify.com/api/<CommandName>?` with `multipart/form-data` body.
**Auth:** `authToken` form field (800-char hex session token). Token survives across navigation — long-lived session cookie equivalent.

| Command | Method | Status | Hits | Params observed | Auth |
|---------|--------|--------|------|-----------------|------|
| `ReconnectApp` | POST | 200 | 5 | `policyIDList`, `authToken`, `referer` | auth-required |
| `Search` | POST | 200 | 3 | `hash`, `authToken` | auth-required |
| `AuthenticatePusher` | POST | 200 | 6 | `socket_id`, `channel_name`, `authToken` | auth-required |
| `OpenInitialSettingsPage` | POST | 200 | 2 | `authToken`, `referer` | auth-required |
| `GetReportPrivateNote` | POST | 200 | 1 | `reportID`, `authToken` | auth-required |
| `PusherPing` | POST | 200 | 1 | `pingID`, `pingTimestamp`, `pusherSocketID`, `authToken` | auth-required |
| `Log` | POST | 200 | 19 | telemetry (skip) | auth-required |
| `Graphite` | POST | 200 | 9 | metrics (skip) | auth-required |
| `fl` | - | 0 | 12 | fl (skip — analytics beacon) | - |

**Total: 6 meaningful commands captured.** Many more exist but were not triggered during navigation-only flows (expense creation, report submission, policy updates).

## Coverage Analysis

**What was exercised:** App reconnect, expense search, report detail view, workspace enumeration, settings initialization.

**What was NOT exercised (inferred from research):**
- Expense creation: `RequestMoney`, `CreateTransaction`, `StartSubmitReport`, `OpenMoneyRequestPage`
- Receipt upload / SmartScan: `StartSplitBill`, `CompleteSplitBill`, `ReplaceReceipt`
- Report submission: `SubmitReport`, `ApproveMoneyRequest`, `PayMoneyRequest`
- Category/tag management: `UpdateWorkspaceCategories`, `UpdatePolicyTags`
- The full Integration Server surface: `Report Exporter`, `Downloader`, `Policy Creator`, `Advanced Employee Updater`, `Report Creator`, `Expense Rules Creator` — these live at `integrations.expensify.com/Integration-Server/ExpensifyIntegrations` and are orthogonal to new.expensify.com's internal API

## Rate Limiting Events
None encountered. Six navigations at ~3s spacing on a logged-in session.

## Authentication Context
- **Transfer method:** Headed login (user logged in via agent-browser headed session)
- **Auth scheme:** Long-lived `authToken` form field (800-char hex) sent in every `/api/<Command>` body. Token is conceptually equivalent to a session cookie but passed explicitly.
- **Persistence target:** `authToken` + `email` can be persisted to a config file (in keychain / ~/.config/expensify/session.json) after login to enable non-interactive CLI invocation.
- **Session state file:** `$DISCOVERY_DIR/session-state.json` — contains cookies/tokens. **Will be removed before archiving** per Phase 5.6 contract.

## Bundle Extraction
Not run. The interactive sniff captured enough signal for the user's primary goal (file expense reports), and bundle extraction adds endpoint paths without response shapes. The generator spec will be written from a blend of: sniff-captured commands, my Phase 1 research of the documented Integration Server, and the two community MCPs (primrose + agenticledger) which collectively enumerate ~35 commands.

## Spec Strategy

Because Expensify's primary automation path (Integration Server) has no OpenAPI spec and the user's goal (filing expenses) requires the internal web API, I will author a **synthetic internal YAML spec** (`kind: synthetic`) that unifies both surfaces:

1. **Section A: Integration Server commands** — documented at integrations.expensify.com/Integration-Server/doc/. Partner credentials (`EXPENSIFY_PARTNER_USER_ID` + `EXPENSIFY_PARTNER_USER_SECRET`).
2. **Section B: New Expensify internal commands** — for filing/submitting/viewing. Session auth (`authToken`) captured via `expensify auth login` flow.

The CLI will support both auth modes. Users who only want to file expenses can use `auth login` (cookie/token path). Users who want admin/export can add `auth set-keys` for the Integration Server path.
