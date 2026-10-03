# PROPHIT deployment

The app uses static HTML/CSS/JavaScript and a separate Go/Gin API. PostgreSQL is the production database; SQLite is development-only. No Redis, frontend build, or npm dependency manifest is required.

## Configure and run

1. Copy .env.example to .env in the repository root. Generate a unique JWT secret of at least 32 characters and a unique database password. Never commit environment files. Rotate any credentials previously used from Git history; removing files does not revoke them.
2. Set ALLOWED_ORIGINS to the exact frontend origins, and APP_URL to the frontend origin for password-reset emails. TRUSTED_PROXIES must list only actual reverse-proxy addresses; empty means forwarded IP headers are ignored.
3. Run `docker compose up -d --build` from the root. The API binds host loopback port 8080. PostgreSQL is private to Compose and persists in profhit_pgdata. Database health gates API startup; both services restart unless stopped.
4. Put a TLS reverse proxy in front of the API and serve the frontend separately. GET /api/health checks database connectivity. The API does not serve HTML.
5. Keep GIN_MODE=release, BETA_MODE=false, and SEED_DEMO_DATA=false in production. Simulator signing and known-password demo accounts are prohibited in release mode.

The backend-only Compose file needs its own backend/.env. Both Dockerfiles use Go 1.26.8, build without CGO, and run as an unprivileged user. backend/render.yaml is the alternative Render blueprint.

## Database and migrations

DATABASE_URL takes precedence over DB_* settings. For individual PostgreSQL settings, DB_SSLMODE defaults to require. Compose disables database TLS only on its private container network; use TLS for remote connections.

Startup calls the same config.Migrate function used by integration tests, including payments, revocations, and notifications. Migration errors stop startup. Back up and test on a staging database copy first. Existing duplicate referrals/predictions/payment IDs, invalid balances, or orphan references may prevent constraints from being created. Investigate and reconcile records; do not silently delete them or fabricate accounting entries.

For SQLite development, copy the example to backend/.env, set a fresh JWT secret, GIN_MODE=debug and USE_SQLITE=true, leave DATABASE_URL empty, then run `go run .` from backend. It creates profhit.db in the current directory with foreign keys enabled and one connection. SEED_DEMO_DATA=true is only for isolated local development.

## Frontend and integrations

Local pages default to http://localhost:8080/api; deployed pages default to https://profhit.onrender.com/api. To use another API, add `<meta name="api-base" content="https://your-api.example/api">` to each served page. Allow that frontend origin in CORS.

- Prediction coin purchase/order/verification/webhook endpoints return 410 and never contact Razorpay. Historical payment records remain intact and need separate financial reconciliation. Approved future advertiser/subscription payments must use a separate economy and staging certification.
- HyperVerge needs app credentials, workflow, provider URL, and webhook secret. Validate the callback schema/signature against the merchant account in staging. Local retry/signature tests do not certify a live provider integration. Missing credentials do not produce fake success.
- Google Sign-In uses `GOOGLE_CLIENT_ID`, which `/api/auth/config` exposes as the public browser client ID. In Google Auth Platform, create a Web application client and register the exact staging origin plus each intentional local development origin. This implementation uses Google Identity Services' browser credential callback, so it does not require a redirect URI. Missing configuration displays an explicit email-login alternative. Certify new-user, existing-user, wrong-audience, expired-token, logout and 2FA flows with the real staging client before release.
- Reset email needs `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, optional `SMTP_SENDER_EMAIL`, and optional `SMTP_SENDER_NAME`. For Gmail, use a dedicated staging mailbox with 2-Step Verification and an App Password; never use the mailbox's normal password. Set `APP_URL` to the exact staging frontend origin so reset links return to the correct deployment. Certify connection, delivery, link use, one-time/expiry/replacement behavior and invalid-credential handling with the staging mailbox.
- Authenticator 2FA is self-service in Settings. Enrollment requires the current password, returns an `otpauth://` provisioning URI and QR code, and becomes active only after a valid 6-digit TOTP. Enabling or disabling rotates the account token version and returns a replacement session. Disabling requires a current TOTP. Recovery codes are not implemented, so document the support recovery process before production rollout.
- News uses NEWS_API_KEY. Fresh articles become deduplicated editorial drafts with URL and published timestamp. Review measurable rules and the approved result source before publication. No API keys means explicit unavailable news, never invented headlines.
- Set VOUCHER_ENCRYPTION_KEY to a base64-encoded 32-byte secret for AES-GCM voucher storage. Keep it in a secrets manager with backup/rotation planning; losing it makes stored vouchers unreadable. Configure TLS SMTP for delivery and validate actual official sourcing and inbox delivery in staging.
- HyperVerge document approval alone cannot authorize redemption. Map verified phone OTP and email confirmation to the same current KYC record only after certifying the provider workflow. There is no client-controlled verification route. This adapter is still pending.

The per-IP rate limiter is process-local. Multiple API replicas need shared enforcement at a reverse proxy/gateway before scaling. Jobs use database claims/constraints for reward and reminder idempotency. Expiry, reminders, and referral payouts run hourly; market transitions run every minute. Daily limits use UTC.

## Verification

From backend: `go fmt ./...`, `go vet ./...`, `go build ./...`, `go test ./...`, and `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`.
From root: `node scripts/check-frontend.mjs`.

Run PostgreSQL concurrency, Docker startup/health, provider sandbox flows, and authenticated browser checks before production. Windows race tests need CGO and a compatible C compiler. Tests do not deploy, rotate credentials, or execute live payments.

## Safe PDF economy transition

Back up the database and rehearse on an isolated PostgreSQL copy. Preserve PaymentTransaction, WalletLedger, prediction amount/potential, referral relationships and achievement claims. Schema changes add prediction streaks, original-batch redemption allocations, typed rules/evidence, challenge links, voucher delivery fields and economy migration records. The old positive prediction-amount constraint is removed without deleting submissions. Startup does not shorten existing expiries.

From backend, `go run ./cmd/reconcile-expiry` prints a read-only old/new expiry preview. Review original earned timestamps, ledger/batch totals and balances that would already be expired. After explicit operator approval, `go run ./cmd/reconcile-expiry -apply -actor <operator-id>` atomically records `pdf-six-month-v1` plus each old/new expiry, then shortens dates without rewriting credits. Repetition is idempotent. The regular expiry service posts the corresponding ledger debit once. Production application remains a separate gate after preview, backup and PostgreSQL rehearsal.

Review historical non-UTC SQLite text timestamps separately. New writes/query cutoffs are UTC, but old dates are not silently rewritten. Existing prediction potential snapshots and paid rewards remain historical promises. Markets with predictions cannot use the no-activity rule editor: reconcile published rules, sources and outstanding obligations explicitly. New participation on unreviewed legacy markets fails closed.

Pending signup/KYC/deposit referral events remain historical records but cannot be paid by the new first-prediction-only job. Review historical paid bonuses against the 20-cap; do not automatically claw back credits. Historical redemptions without original-batch allocations require reconciliation before refunds. Never mint replacement coins with a fresh expiry. Old challenges without a market link remain reconciliation items.

Test concurrent expiry/refund/settlement and multiple job processes on PostgreSQL. Validate this transition against a production-data copy before deployment; no production transition was performed in the local audit.
