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

- Razorpay needs key ID, key secret, and webhook secret. Configure captured-payment webhooks at /api/webhooks/razorpay. Whole INR deposits from 10 to 100000 credit 1 PTS per INR. Verification fetches provider capture, amount, and order details; both verification channels share atomic settlement.
- HyperVerge needs app credentials, workflow, provider URL, and webhook secret. Validate the callback schema/signature against the merchant account in staging. Local retry/signature tests do not certify a live provider integration. Missing credentials do not produce fake success.
- Google Sign-In uses GOOGLE_CLIENT_ID exposed by /api/auth/config. Register actual frontend origins with Google. Missing configuration displays an explicit email-login alternative.
- Reset email needs SMTP_HOST, port, username, password, and sender email. Delivery failures are logged. Test a real reset email in staging and set APP_URL correctly.
- News uses NEWS_API_KEY. Wild-card generation creates editorial drafts, never automatically publishes vague questions.

The per-IP rate limiter is process-local. Multiple API replicas need shared enforcement at a reverse proxy/gateway before scaling. Jobs use database claims/constraints for reward and reminder idempotency. Expiry, reminders, and referral payouts run hourly; market transitions run every minute. Daily limits use UTC.

## Verification

From backend: `go fmt ./...`, `go vet ./...`, `go build ./...`, `go test ./...`, and `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`.
From root: `node scripts/check-frontend.mjs`.

Run PostgreSQL concurrency, Docker startup/health, provider sandbox flows, and authenticated browser checks before production. Windows race tests need CGO and a compatible C compiler. Tests do not deploy, rotate credentials, or execute live payments.
