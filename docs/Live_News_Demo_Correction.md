# Live-news demo correction — verified October 4, 2026

Public demo: https://profhit.vercel.app

This is the existing Go/Gin application and original frontend, served together by a Vercel Docker Function with real Neon PostgreSQL in Singapore. Public access does not require a Vercel account. Application authentication and roles remain enforced. No real-money transactions, emails or identity-provider verification were triggered.

## Correction and root causes

| Issue | Root cause and correction |
| --- | --- |
| Sparse cards and regression from the existing design | Dashboard retained the older renderer, while the public page lacked the native card stylesheet. Both feeds now use the existing card/layout styles and one renderer; obsolete controls and their listener were removed. |
| Blank icons/profile control | The remote icon loader requested another stylesheet disallowed by CSP. Existing Phosphor fonts/styles are now packaged locally. Profile has a visible label; CSP was preserved. |
| PTS, probability limitation text and giant empty chart | Static copy and unused probability/chart structure remained from the old market view. User-facing currency is Coins; unavailable probability content is omitted. |
| Answer clipping and unreadable long choices | Fixed choice layout squeezed text. Choices wrap, and four-choice questions stack within the existing sidebar; prediction controls are disabled during loading and private preview. |
| Sidebar balance stayed stale after settlement | The header subscribed to wallet events, but the sidebar did not. The sidebar now subscribes and removes its listener on disconnect; daily-login rewards refresh both balances. A regression reproduces 90 → 110 after a wallet event. |
| Generic fixture filler | Bulk automatic curation prioritized quantity. The publisher now requires an explicitly authored reviewed manifest. Fifty-three source-backed questions replace the visible filler feed. |
| Publication gate bypass | An alternate Draft → Live transition allowed bypassing publication review. Publication is now transactional and requires all six checks, future cutoff, trusted result source, typed rules and relevance rationale. Draft → Live is rejected. |
| Draft source edits failed to persist | Source/provenance fields were omitted from draft updates. Authenticated draft editing now validates and saves them. |
| Cancellation accounting | New void/refund operation locks market/wallet rows, refunds each actual entry once, preserves history, records audit/notification/ledger entries, and excludes voided entries from active counts and daily quota. PostgreSQL concurrency tests cover duplicate refund attempts. |

## Content snapshot

The public API returned 53 reviewed playable opportunities, IDs 358–410, with no audit records counted. The manifest is `editorial-demo-catalog.json`.

| Category | Count |
| --- | ---: |
| Politics | 3 |
| Geopolitics | 6 |
| Sports | 6 |
| Technology | 10 |
| Financial Markets | 11 |
| Entertainment | 8 |
| Weather | 5 |
| Wild Card | 4 |

Each question has distinct context, source, future cutoff, expected resolution time, measurable rule, fixed cost/reward and review rationale. Conditional launches are forecasts, not fabricated confirmed announcements. Official future events are not mislabeled as dated news articles. BBC RSS ingestion is active; news candidates need review before publication. GNews/private provider credentials are not enabled.

The count is a time-stamped snapshot. An operator must replenish reviewed opportunities as deadlines expire. “Live” means open to actual entries, not a claim that the outcome already exists or that every source is streamed automatically. Counts reflect real submissions; no odds, probabilities or participants were invented.

## Public browser and API verification

Fresh public tab and an HTTP request without authentication opened the homepage (200). The native feed, search empty state, restoring results, question/detail navigation and 375-pixel mobile layout were inspected. Normal registered disposable accounts logged in through the actual browser form. No token was injected into the browser.

The Dharapuram market changed from “Be the first to predict” to one player after a browser entry. Balance changed 110 → 100 after the 10-Coin cost. Excluded operational market 411 received three actual submissions and locked; genuine provider evidence determined Up. Exactly one winner received 20 Coins, changing 90 → 110. The browser updated the outcome and header without reloading; the sidebar discrepancy was reproduced and corrected.

Admin editing saved a private draft; protected preview displayed four long choices, cost and reward, with play disabled and no comments endpoint error. Create/publish/feature used the actual protected endpoints for the 53 reviewed markets. Scheduled opening, pause/resume, lock and resolve used actual database state. Empty excluded audit 413 was cancelled through the normal void endpoint with zero entries; daily category limits remained intact. Void/refund concurrency and idempotence were also tested against PostgreSQL.

| Negative test | Actual result |
| --- | --- |
| Anonymous admin access | 401 — `Authorization header required` |
| Regular user admin/preview | 403 — `Access denied: insufficient permissions` |
| Invalid session | 401 — `Invalid or expired session` |
| Public draft access | 404 — `Market not found` |
| Publish without editorial review | 400 — `six editorial checks and a specific relevance rationale are required` |
| Direct Draft → Live | 400 — `Illegal market state transition` |
| Wrong entry cost | 400 — `Confirm the published virtual coin entry cost; cash and arbitrary stakes are not accepted` |
| Closed market entry | 400 — `This market is no longer accepting predictions` |
| Repeat settlement | 400 — `Market must be Locked or Awaiting Resolution before it can be resolved. Current status: Resolved`; balance unchanged |

The earlier all-page audit is not a substitute for verification of every external provider. This correction re-tested the public feed, authenticated market flow, wallet, notifications, leaderboard, admin draft controls and responsive layout. Final public/player console logs contained zero warnings or errors. Negative API errors above were expected assertions; they are not hidden successful responses. Two exploratory requests initially used incorrect route paths and returned `Route not found`; corrected requests used the existing `/api/me` and `/api/markets/:id/resolve` contracts.

The final live-wallet repeat used fresh normally registered accounts after existing daily category limits rejected reusing prior audit players. Excluded audit market 414 received three actual entries before cutoff. The retained baseline was USD 84,828, and the first eligible retained provider quote was USD 84,821 at 2026-10-04T04:08:20Z, after the declared 04:07:42.6277304Z observation start. Down therefore won. Settlement paid exactly one winner 20 Coins; its 100-Coin balance became 120 in the immutable ledger. Both browser balances and the resolved state changed automatically without a reload. The notification showed the same reward and the leaderboard showed 120 Coins and rank 2. A separate +10 daily-login grant is recorded independently from the reward. Audit markets 411–414 are excluded from the 53-count; draft 412 stays unpublished and 413 was cancelled empty.

## Exact validation commands

Go commands ran from `backend`; frontend/patch commands ran from the repository root. All exited 0.

```text
go fmt ./...
go vet ./...
go build ./...
go test ./...
go mod verify
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
node scripts/check-frontend.mjs
git diff --check
```

Module verification returned `all modules verified`. Frontend checks passed 27 HTML pages and 27 script checks, local links/modules, markup structure, IDs, escaping, actual status rendering, countdowns and live-sidebar regression. Vulnerability scan reported zero reachable/imported-package vulnerabilities, plus one advisory in an unused required module; it exited 0.

Linux CI passed on deployed code revision `98ef85a3fdf868efaf6899e1a42a6e3afcd4fa8c`: https://github.com/SoorejS/profhit/actions/runs/37176457802. It includes PostgreSQL 18, `go test -race -count=1 ./...`, production Docker build, non-root startup/schema and public API checks.

Public URL points to deployment `dpl_2qSp6K2j3BB15cPjepE2TpCnKd4W`. Deployment health was checked before promotion; anonymous homepage returned 200 afterward and the API still returned 53 reviewed opportunities with zero audit or unreviewed entries. Final leaderboard labels say Coin Balance and Total Coins. The deployment package includes only approved public assets and backend source; ignored caches, credentials and audit artifacts were excluded. Use Vercel CLI 62.2+ and the explicit existing team scope when packaging/deploying.

## Remaining gates

This is a working virtual-coin client demo, not a production-readiness claim. Human admin handoff, ongoing editorial/result operations, historical credential revocation, private-provider staging, production-data reconciliation, email/phone/age verification, voucher fulfillment/delivery and a separately authorized production release remain gates.

Automatic approval review rejected a broad cancellation/refund request because it would make irreversible changes beyond an explicit approved market list. It was not executed. Sixty individually inspected old filler markets were instead reversibly paused, preserving submissions and balances. Old paused Bitcoin market 130 retains one actual pending entry and needs its original evidence-based result or a separately scoped cancellation decision. This is separate from the new 53-market feed and the excluded audit fixtures.

Proof artifacts are retained locally under ignored `audit-artifacts`: publication log, feed snapshot, provider responses/timestamps, settlement/ledger/notification results, access test results, frontend screenshots and command logs. Sensitive session credentials are DPAPI-protected and excluded from source/deployment.
