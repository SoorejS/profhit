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

The feed counted 66 genuine future opportunities on October 3, 2026 UTC, from ESPN schedules, National Weather Service forecasts and CoinGecko quotes. `scripts/curate-demo.mjs` obtains actual source data, records deadlines and measurable rules, creates drafts and publishes through authenticated admin endpoints. Sources without an actual publication date are labeled official future events; no date is invented. Audit markets are explicitly flagged and excluded from the genuine count.

BBC RSS feeds ingest only articles with actual publication dates within 24 hours. GNews and other existing adapters remain configurable; GNews was not enabled because no credential was supplied. Articles become playable only after a measurable result rule and editorial approval. Eight categories are supported by the editor, filters and typed rules; the published catalog contains all eight categories. Additional curated events use the official Python release schedule, European Council calendar, FEC election calendar and Nobel announcement schedule. These are future opportunities, not invented dated news articles. The public catalog in `docs/genuine-demo-current-events.json` records their sources and rules.

Admin supports create, edit unpublished rules, publish, schedule, lock and resolve. Results require an approved source and an actual observation timestamp at or after cutoff. Official sports/weather/price questions in this curated catalog require an administrator to review their published result source; they are not falsely described as automatically resolved. Missing results remain awaiting resolution.

WebSockets are retained. A database-backed 10-second polling signal ensures wallet, counts, market lifecycle, leaderboard and notifications update across separate Vercel function instances. Request-time lifecycle reconciliation prevents reliance on an always-running timer when functions scale to zero.

The 50-market threshold is a snapshot, not a perpetual inventory guarantee. Run live curation and review regularly as deadlines expire. No odds or participant counts are synthesized.

## Verification

Local formatting, vet, build, unit tests, module verification, frontend syntax/link/escaping/countdown checks and patch whitespace checks passed. `govulncheck@latest` reported zero reachable or imported-package vulnerabilities, with one unused-module advisory. Linux PostgreSQL/race/container CI passed for source revision `3d87487` at https://github.com/SoorejS/profhit/actions/runs/37146196179. It exercises the Vercel Dockerfile, public frontend, schema verification, coin transactions and shared rate limits.

Public browser verification passed for normal login, automatic Scheduled → Live opening, a 10-coin debit (110 → 100), a live prediction-count update to three, automatic locking and automatic display of the resolved result. An actual Bitcoin quote of USD 84,927 at 2026-10-03T18:45:10Z was below the declared USD 84,940 baseline. The quote was inside the published observation window; exactly one Down prediction won. Its wallet changed 90 → 110 on settlement, then 120 after a separately recorded 10-coin daily login. The browser verified the debit/win ledger, notification and first-place leaderboard entry. The audit market is excluded from the 66-market count. Admin browser creation saved a genuine Technology draft, and authenticated publication made it live.

Actual public HTTP checks: health/live feed/live state 200; unauthenticated wallet/admin access 401; regular-user admin access 403; invalid token 401; wrong coin cost 400; closed-market prediction 400; repeated settlement 400 with no second payment; unknown API route, environment file and backend source 404. All 66 feed items had `is_demo=false`.

Errors found and fixed: old CLI deployed only static assets (`/api/health` 404); cold-start migration exceeded the approximately 28.7-second startup budget (`FUNCTION_INVOCATION_FAILED` 500); cross-region requests during initial curation produced `[API Timeout] /live-feed?section=latest&limit=60&offset=0&search=&category=`; countdown minute conversion was six times too large; public text had invalid UTF-8 separators; proxy rate limits treated visitors as one address; the admin editor omitted the answer-range width needed for range questions. The corrected deployment uses the new CLI, offline migration, Singapore compute/database, a corrected timer regression, UTF-8 text and durable limits. Console errors after the corrected public coin-flow deployment were zero.

This is a virtual-coin client demo. It is not a production-readiness declaration. Human administrator handoff, credential revocation for older environments, real-provider staging, production-data reconciliation, email/phone/age verification, voucher fulfillment and any production release remain separate gates.
