# PROPHIT Vercel client demo

Public URL: https://profhit.vercel.app

The existing Go/Gin application serves the existing HTML/CSS/JS frontend and `/api` from one Vercel Docker Function. Neon PostgreSQL was provisioned through the Vercel Marketplace in Singapore. Function region is `sin1`; no Render dependency remains in the browser/API path.

## Deployment

Use Vercel CLI 62.2 or later. The older CLI did not recognize Docker services and deployed static files without the API.

`vercel.json` routes to `Dockerfile.vercel`. The container runs as UID 10001, includes only public assets in `/app/public`, and refuses dotfiles, directory listings and HTML fallbacks for unknown API routes. Build a clean source package with `scripts/package-vercel.ps1`; deploying the workspace directly can upload local caches.

Secrets belong only in the hosting environment. Never commit environment files or pull production secrets into the deployment package. Marketplace injects the database URLs. Use a new random JWT secret. `SEED_DEMO_DATA=false`, `BETA_MODE=false`, and `VIRTUAL_COIN_DEMO=true`. No payment, email, verification or private news-provider credentials were enabled for this demo.

Run `backend/cmd/migrate` explicitly against the intended demo database before deploying. Use `AUTO_MIGRATE=false` for Vercel startup: full schema introspection over a distant database exceeded the platform startup budget. Startup still verifies required schema and fails closed if it is missing. Migrations share the existing model schema and serialize with a PostgreSQL advisory lock.

`TRUSTED_PROXIES=127.0.0.1/32,::1/128` trusts only the platform container proxy. PostgreSQL-backed rate-limit buckets enforce the same limits across function replicas. Application JWT checks, roles, revoked tokens and token versions remain enforced. Production Vercel SSO protection is disabled; preview protection remains enabled.

## Coins, markets and results

Registration grants 100 welcome coins; the existing daily-login grant is 10 coins. Curated demo markets cost 10 virtual coins. The entry cost is published per market; legacy zero-cost markets remain free. Arbitrary stakes and cash payments are rejected. Submission, FIFO coin consumption, wallet ledger and market volume commit atomically. Existing typed fixed rewards and settlement transactions are preserved.

The corrected feed counted 53 reviewed future opportunities on October 4, 2026 UTC. The editorial manifest is `docs/editorial-demo-catalog.json`: Politics 3, Geopolitics 6, Sports 6, Technology 10, Financial Markets 11, Entertainment 8, Weather 5 and Wild Card 4. These are individually written forecasts based on actual sources and future outcomes. They are not all dated news articles. `scripts/curate-demo.mjs` validates the explicit manifest and creates drafts through protected admin endpoints only when publication is requested. It no longer mass-generates generic fixtures. Audit markets are flagged and excluded from the genuine count. Do not re-publish the same manifest blindly; review existing markets first.

BBC RSS feeds ingest only articles with actual publication dates within 24 hours. GNews and other existing adapters remain configurable; GNews was not enabled because no credential was supplied. Articles become playable only after measurable result rules and editorial approval. Official future schedules and current public provider data are labeled separately, with no invented article date. News candidates remain private drafts until reviewed; source ingestion alone does not mean a playable market exists.

Admin supports create, edit unpublished rules, protected preview, schedule, publish, feature, pause/resume, lock, resolve and void/refund. Publication requires all six editorial checks and a specific relevance rationale; direct state transitions cannot bypass publication review. Results require an approved source and an actual observation timestamp at or after cutoff. Catalog results require a human administrator to review actual evidence; they are not described as automatically resolved. Missing results remain awaiting resolution. Cancellation preserves prediction history and refunds actual entry Coins exactly once.

The original PROPHIT typography, layout, navigation, components and colors remain the baseline. Public and authenticated feeds use the same native cards. Cards display real context, source, cutoff, resolution time, entry Coins, fixed reward and actual participation. Zero participation says “Be the first to predict.” The probability-unavailable message and empty chart are removed. Long answer choices wrap without clipping; multi-choice answers stack in the existing trade sidebar. Icons are packaged locally to avoid the icon loader requesting a stylesheet forbidden by the existing CSP. Header and sidebar balances both subscribe to real wallet updates.

WebSockets are retained. A database-backed 10-second polling signal ensures wallet, counts, market lifecycle, leaderboard and notifications update across separate Vercel function instances. Request-time lifecycle reconciliation prevents reliance on an always-running timer when functions scale to zero.

The 50-market threshold is a snapshot, not a perpetual inventory guarantee. Run live curation and review regularly as deadlines expire. No odds or participant counts are synthesized.

## Verification

Local formatting, vet, build, unit tests, module verification, frontend syntax/link/escaping/countdown/live-wallet checks and patch whitespace checks passed again. `govulncheck@latest` exited 0, with zero reachable or imported-package vulnerabilities and one unused-module advisory. Linux PostgreSQL/race/container CI passed for deployed revision `98ef85a3fdf868efaf6899e1a42a6e3afcd4fa8c` at https://github.com/SoorejS/profhit/actions/runs/37176457802. It exercises PostgreSQL 18, `go test -race -count=1 ./...`, the production container, schema verification, public API, coin transactions and shared rate limits. Public URL points to deployment `dpl_2qSp6K2j3BB15cPjepE2TpCnKd4W`.

The corrected public browser flow verified normal email login, a Dharapuram prediction costing 10 Coins (110 → 100), and participation changing to one real player. Excluded audit market 411 verified three actual entries, automatic locking, a genuine CoinGecko quote of USD 84,865 observed at 2026-10-04T03:43:30Z versus the retained USD 84,806 baseline, one Up winner paid 20 Coins (90 → 110), the ledger and win notification. The open browser displayed the resolved outcome and new header balance without a reload. This exposed a stale sidebar balance, fixed with a live subscription and regression test. Final excluded audit 414 used fresh registered accounts and a real USD 84,821 quote at 2026-10-04T04:08:20Z versus baseline USD 84,828. One Down winner received 20 Coins; both header and sidebar updated 100 → 120 without reloading. The wallet ledger, notification and leaderboard matched. Admin browser editing and protected preview passed, including four long answer choices. Desktop and 375-pixel mobile views were inspected. No console warnings/errors were recorded in the final public/player browser flows.

Actual public HTTP checks: anonymous homepage/health/live feed/live state 200; unauthenticated admin access 401; regular-user admin and private preview access 403; invalid token 401; public draft access 404; unreviewed publication and draft-to-live transition 400; wrong coin cost 400; closed-market prediction 400; repeated settlement 400 with no second payment. All 53 feed items had `is_demo=false` and recorded editorial review. Scheduled opening, pause/resume and cancellation of newly created empty excluded audit market 413 were verified. Existing daily category limits remained enforced.

Earlier deployment fixes remain in place: modern Docker-capable CLI, explicit offline migration, Singapore compute/database, correct countdown conversion, UTF-8 source text and durable rate limits. This correction also removed the stale dashboard renderer/listener, fixed missing public card CSS and hidden legacy controls, added the publication gate and source-edit persistence, and repaired missing icons, answer clipping and live sidebar balance. Read `Live_News_Demo_Correction.md` for current evidence and limitations.

Sixty explicitly inspected older unreviewed filler markets were reversibly paused, preserving predictions and balances. Automatic approval review rejected broad bulk cancellation/refunds as irreversible changes outside a sufficiently explicit market list. The broad cancellation was not run. One older paused Bitcoin market (130) retains a real pending entry; its original result obligation still needs reviewed evidence or explicit scoped cancellation. Newly created empty excluded audit market 413 was separately cancelled with zero refunded players.

This is a virtual-coin client demo. It is not a production-readiness declaration. Human administrator handoff, credential revocation for older environments, real-provider staging, production-data reconciliation, email/phone/age verification, voucher fulfillment and any production release remain separate gates.
