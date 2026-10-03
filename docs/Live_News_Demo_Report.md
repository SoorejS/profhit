# PROPHIT live-news demo verification

Status: core implementation, local checks and Linux PostgreSQL/race/container verification passed. **Client-demo acceptance has not passed.** No claim of production readiness. Updated October 3, 2026.

## 1. Existing PROPHIT Features Reused

Existing authentication and roles, Market model, proposal/review queue, category/difficulty answer validation, free PredictionSubmission flow, wallet ledger, rewards, leaderboard, notifications, transactional settlement, and authenticated WebSocket hub remain the application architecture. The existing settlement transaction is shared by manual and weather-provider resolution.

## 2. New Live-News Features

Persisted news provenance and multiple sources, canonical URLs, publication/discovery times, fingerprint and title-token deduplication, a database ingestion lease, quota schedule and backoff, editorial drafts, optional structured AI candidates, source-linked validation, public live feed, search, category filters, latest/breaking/trending/closing-soon/personalized sections, actual counts, pagination, and admin news controls. Drafts and development fixtures never count toward the live-news target.

AI output is not a result authority. Unsupported or ambiguous events remain unpublished. Token similarity is a heuristic, not certified semantic entity recognition.

## 3. News Providers

GNews is the configured primary adapter: four category requests per refresh, up to 50 articles per category by default, only English articles published within 24 hours. The selected GNews plan must support this page size. One hourly refresh uses up to 96 requests/day; deployment restarts, manual generation and provider changes must be considered when budgeting. Approved configured RSS/Atom feeds are an optional fallback; undated entries are rejected. No fabricated fallback data.

Optional structured generation uses an explicitly configured OpenAI model/key and strict structured output, followed by backend validation and editorial approval. It is disabled by default. Neither real GNews nor OpenAI requests have been verified yet.

Primary references: [GNews top headlines](https://docs.gnews.io/endpoints/top-headlines-endpoint), [OpenAI structured outputs](https://developers.openai.com/api/docs/guides/structured-outputs), [OpenWeather current observations](https://openweathermap.org/current).

## 4. Number of Live News Items

Verified disposable local database: **0 current provider-derived news events**. No live provider credentials were used locally. The live deployment count is unverified; Render's existing backend failed startup.

## 5. Number of Playable Predictions

Verified disposable local live-news feed: **0 genuine playable predictions**; `target_met=false`. The acceptance target is at least 50. An isolated regression test verifies exactly 50 eligible records out of 55 test fixtures, excluding demo, stale, draft, locked and incomplete entries. These fixtures are not live news and were not inserted into the browser application's feed.

## 6. Categories

Sports, Weather, Politics, Entertainment, Financial Markets and Wild Card use existing rules and payouts. Category availability does not mean a live provider has supplied suitable future events in each category.

## 7. Prediction Types

Existing 18 category/difficulty formats are preserved: binary/winner/direction, constrained ranges and nominees, exact or closest measurable answers. Rules, source, units and deadlines must be reviewed before publication. No odds or outcomes are generated for display.

## 8. Real-Time Features

The existing authenticated hub broadcasts news-event changes and market lifecycle/activity events. The feed refreshes on these events. Visitors use a 60-second refresh without bypassing WebSocket authentication. CSP and API/WS fallback now point to the verified Render service, `profhit-1.onrender.com`. Live deployed WS remains unverified.

## 9. User-Created Predictions

The existing proposal form validates the published formats. Duplicate submissions return HTTP 409 and the existing title/ID. Public published predictions receive a details link; unpublished duplicates are shown as awaiting review. PostgreSQL advisory transaction locks serialize duplicate creation by category. Disposable local browser submission and duplicate rejection passed.

## 10. Resolution System

OpenWeather supports reviewed Easy Weather binary markets with exact coordinates, units, threshold, future cutoff and a 15-minute observation window. Missing, stale or mismatched observations cannot produce a guessed payout. Provider evidence contains the actual observation timestamp and measurement, without the API key. The shared transaction enforces one settlement and ledger consistency; a repeated weather settlement regression test passes.

Sports and financial automatic result adapters are **not complete**. They require an approved provider, concrete event identifiers and staging contract tests. Existing approved result-domain validation and manual evidence settlement are not equivalent to live provider integration. Manual review remains necessary for other categories and unsupported weather formats.

## 11. Multi-User / Load Results

Measured local opt-in test, in-memory SQLite, one database connection, actual API/router/JWT/WS, all clients sharing one loopback IP. WebSocket handshakes ramp at concurrency 32; HTTP transport limits concurrent connections to 100. These are not PostgreSQL or Render capacity measurements.

| Users/connected sockets | Accepted predictions | HTTP 429 | Other failures | HTTP p50 | HTTP p95 | Workload elapsed |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 100 | 30 | 70 | 0 | 82.721 ms | 150.668 ms | 0.207 s |
| 500 | 30 | 470 | 0 | 785.099 ms | 1131.369 ms | 1.536 s |
| 1000 | 30 | 970 | 0 | 1145.917 ms | 1572.998 ms | 2.669 s |

The existing per-IP limit of 30 submissions/minute stayed enabled. Broadcast receipt was checked on all connected sockets; accepted submission counts and ledger invariants agreed. p50/p95 include client queuing. Heap after workload was approximately 9.45/47.54/56.30 MB; these are Go heap values, not whole-process RAM. CPU and deployed database query latency were not measured. An earlier unbounded connection burst reached Windows connection refusals; the final results above use the stated ramp.

## 12. Deployment

Existing frontend: https://profhit.vercel.app (previous deployment, not yet this sprint).

Verified Render backend service: `profhit-1`, `srv-d9q69gqd0e5s73927l2g`, https://profhit-1.onrender.com, Docker, Oregon, Free. Last observed deployment at commit `08bcf6b` failed after building successfully with the exact error: `JWT_SECRET must be a unique random secret of at least 32 characters`. No security check was weakened.

Created dedicated free Render PostgreSQL 18 demo database: `profhit-demo-db`, `dpg-db0ig7psrm7s73fohhfg-a`, Oregon. Render status **Available** is verified; application connection and migration remain pending. Render states expiry November 2, 2026. This is a temporary demo database, not the production database gate. Render also displays a pending-invoice banner; billing impact on the demo has not been established.

The user must store the database URL, a fresh JWT secret and provider keys directly in the Render environment. No credentials were copied into repository files.

Linux CI **passed** at code commit `29a14bc`: formatting/module/vet/build checks, full `go test -race -count=1 ./...` with PostgreSQL required rather than silently skipped, frontend checks, Docker image build, and actual non-root container startup with PostgreSQL plus health/live-feed API contract checks. [Verified workflow run](https://github.com/SoorejS/profhit/actions/runs/37136659690). PostgreSQL tests cover wallet, payment idempotency, redemption stock, market settlement, ingestion lease and simultaneous duplicate proposal creation. This certifies those test cases in CI; it does not certify historical production data or deployed Render capacity.

Updated frontend preview deployed **READY** as `dpl_Bb1ZmWgQs4RNQVMKkKha7VkiwFzq`: https://profhit-h0v3exlyn-soorejs-projects.vercel.app . Browser access redirects to Vercel sign-in because deployment protection is enabled. No access controls were bypassed or disabled. It is not a public client-demo URL. Local Go caches were excluded from frontend deployment after canceling an oversized initial upload.

## 13. Browser Acceptance Results

New local browser checks: public landing, truthful empty counts, loading delay, injected HTTP 503 failure, retry recovery, search, 320/375/1440 widths, expired session redirect, seeded local administrator login, authenticated feed, required proposal fields, proposal submission, duplicate feedback and admin news-control empty state passed. Narrow landing document width matched the viewport, with no horizontal overflow. No new console warnings/errors were returned for these flows.

Exact expected network responses: `/api/live-feed` 200 with empty actual counts; injected `/api/live-feed` 503 with `Disposable audit: injected API failure`; `/api/markets/propose` 200 on creation and 409 on duplicate; `/api/news` 503 while no provider is configured. Actual sourced card rendering, deployed visitor/user flows, live-provider requests and two-browser live participation remain pending. Prior full authenticated audit belongs to `PDF_Reconciliation_Report.md`; it is not a new deployed acceptance pass.

Local screenshots are in ignored `audit-artifacts/pdf-sprint/live-news-mobile.png`, `live-news-desktop.png` and `render-demo-db.png`. The new HTTPS preview is protected by Vercel authentication, so public deployed-browser acceptance remains blocked in addition to backend configuration.

## 14. Errors Found and Fixed

Request-driven provider fetching risked quota exhaustion: moved ingestion behind a persistent worker lease. News drafts could otherwise be mistaken for playables: added strict public-feed eligibility and actual target counts. Duplicate proposals needed a shared transaction and actionable 409 feedback. Alternate sources incorrectly inherited the primary timestamp: each source now keeps its own publication timestamp. Legacy Fast Movers could remain a skeleton: it now displays actual current-feed activity or a clear empty/error state. Public market details now enforce visibility. Frontend API fallback pointed to the wrong Render hostname; HTTPS and secure WS target the verified service. PostgreSQL test skips no longer print connection strings; CI fails when a required test database is unavailable.

## 15. Remaining Issues

Release blockers: Render environment configuration and database connection; real GNews ingestion; editorial approval of at least 50 unique suitable future questions; live sports/financial result adapters and provider staging checks; deployed mobile/desktop and simultaneous-user acceptance; provider-plan quota confirmation and monitoring. PostgreSQL concurrency, race detector and container startup checks have passed in Linux CI. Credential rotation/revocation, production-data reconciliation, phone/email/age-provider staging and voucher delivery remain separate production gates.

Local commands from `backend` unless indicated:

| Command | Observed result |
| --- | --- |
| `go fmt ./...` | Pass; formatted changed Go sources |
| `go vet ./...` | Pass |
| `go build ./...` | Pass |
| `go test ./...` | Pass; PostgreSQL tests skipped because disposable localhost database unavailable; opt-in load test skipped in default suite |
| `go mod verify` | Pass: all modules verified |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Pass: 0 reachable vulnerabilities, 0 imported-package vulnerabilities; 1 required-module vulnerability not called |
| `go test -race ./...` | Local Windows blocked: CGO/GCC unavailable. Linux CI `CGO_ENABLED=1 go test -race -count=1 ./...` passed with required disposable PostgreSQL |
| `node scripts/check-frontend.mjs` (repo root) | Pass: 27 HTML pages, 25 script syntax checks and structural/security regressions |
| `git diff --check` (repo root) | Pass; Git emits Windows line-ending notices |
| `RUN_LIVE_LOAD_TESTS=true go test ./tests -run TestLiveLocalLoad -v -count=1` | Pass for 100/500/1000 scoped local workloads |
| `docker info` | Blocked: Docker engine pipe does not exist; installed Docker Desktop startup attempt did not produce an available engine |
| Linux CI Docker image build and API container startup | Pass against disposable PostgreSQL 18; health and public-feed contracts checked |

## 16. Git Changes

This sprint extends the existing repository. Committed and pushed verification branch: `codex/live-news-demo`. Implementation `8814da2`, whitespace correction `acd383c`, container startup/cache exclusions `29a14bc`. Base local commit is `5bf3d74`; observed origin/main remains `08bcf6b`. The verification branch is not merged into main. No secrets, SQLite databases, binaries, local caches or audit screenshots were committed. Do not claim demo acceptance until actual live-data and deployed-browser gates pass.

## 17. Final Demo URL

**No accepted live-news demo URL yet.** Protected preview: https://profhit-h0v3exlyn-soorejs-projects.vercel.app . Existing frontend https://profhit.vercel.app is not verified to contain this sprint or 50 genuine playables. Local implementation: http://127.0.0.1:5174 . Render backend candidate: https://profhit-1.onrender.com .
