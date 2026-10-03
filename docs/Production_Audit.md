# PROPHIT Production Certification & Stabilization Audit — updated 2 October 2026

## 1. Executive Summary

The PROPHIT application has reached commit `785d7a0` on `main` at repository `https://github.com/SoorejS/profhit`. All repository-side release gates, static frontend checks, Go compilation, vulnerability scans, race detector verification, Docker Compose stack builds, PostgreSQL schema auto-migrations, row-level concurrency tests, financial accounting invariant checks, and database disaster recovery drills have been executed and verified.

**Release Status: NO-GO.** Repository-side release gates were previously passed on `main`. The 2 October authentication update completes the TOTP enrollment/login/disable implementation and local regression flow, but the requested authentication certification is incomplete. Production approval requires external operator action: rotating legacy credentials present in Git history, validating a code from a real authenticator app, and supplying live staging credentials for SMTP and Google OAuth certification, alongside the previously listed provider gates.

---

## 2. Release Gate Classification Matrix

| Gate | Status | Evidence |
| --- | --- | --- |
| Current secret files removed | PASS | `git ls-files '*.env' '*.db' '*.sqlite' '*.exe' '*.pem' '*.key'` verified 0 sensitive tracked files in git index and tree. |
| Historical credentials rotated | BLOCKED | Historical git commits contain legacy values (`JWT_SECRET`, `DB_PASSWORD`, `NEWS_API_KEY`, `GEMINI_API_KEY`, `SMTP_PASSWORD`). Untracked environment retains unrotated legacy keys/placeholders. Operator external revocation & rotation required. |
| Razorpay sandbox | BLOCKED | `RAZORPAY_KEY_ID` and `RAZORPAY_KEY_SECRET` contain placeholder values in configuration. Live provider sandbox cannot be contacted without operator-supplied test credentials. Unit mock and HMAC tests pass. |
| HyperVerge sandbox | BLOCKED | `HYPERVERGE_API_KEY`, `HYPERVERGE_API_SECRET`, and `HYPERVERGE_WORKFLOW_ID` contain placeholder values. Live sandbox cannot be contacted without operator-supplied credentials. Unit retry backoff tests pass. |
| SMTP staging | BLOCKED | Staging requires dedicated test mailbox; current configuration retains legacy personal credentials without external rotation. Safe degradation (HTTP 503) verified. |
| TOTP 2FA | BLOCKED | Local browser enrollment, QR/provisioning, standards-based TOTP verification and login enforcement passed. Final certification requires scanning the QR and entering a code from an operator-controlled authenticator application; no such device/app interaction was available. Recovery codes are not implemented. |
| Google OAuth staging | BLOCKED | Google Client ID unverified against live staging origin; no authorized staging ID token provided for live browser handshake. Unit token parsing tests pass. |
| Production data rehearsal | BLOCKED / N/A | Operator has not supplied a sanitized production database backup. Disposable PostgreSQL 15 auto-migration and pg_dump/restore drills passed. |
| Final staging smoke test | BLOCKED | Full end-to-end journey smoke test requires active staging deployment with the rotated provider credentials listed above. |
| PostgreSQL | PASS | PostgreSQL 15 container verified. 24 tables auto-migrated with indices and foreign keys. 4/4 concurrent money-path tests passed using `clause.Locking{Strength: "UPDATE"}`. |
| Race detector | PASS | `go test -race ./...` executed in Linux container (`golang:1.26.8-alpine` with `gcc` and `musl-dev`). Zero data races detected. |
| Docker | PASS | Multi-stage Docker build `profhit-backend:test` succeeded (Go 1.26.8 builder, Alpine 3.23 runner, unprivileged app user `10001`). Compose stack verified healthy. |
| Wallet invariant | PASS | Invariant verified: $\text{User.Points} = \text{ledger balance sum} = \text{unexpired batch balance sum}$. Zero negative balances, double-spends, or duplicate credits. |
| Backup/restore | PASS | Verified `pg_dump` backup (209 KB), `DROP DATABASE`, `CREATE DATABASE`, and `psql` restore drill with 100% schema, sequence, and data integrity. |
| Pagination | PASS | Bounded pagination contract (`items`, `page`, `page_size`, `total`, `total_pages`) implemented on 7 list endpoints; frontend aggregates queried via `/api/me/stats`. |
| Frontend | PASS | `node scripts/check-frontend.mjs` passed (25 HTML pages, 21 ES modules, 0 syntax/XSS/broken link errors). |
| Vulnerability scan | PASS | `govulncheck ./...` passed with zero reachable vulnerabilities in application code. |

---

## 3. System Architecture & Component Inventory

| Component | Architecture & Certification Findings |
| --- | --- |
| **Frontend** | 25 static HTML pages, CSS design system, native ES modules. Includes auth, dashboard, markets, portfolio, wallet/KYC, rewards, profile, leaderboard, notifications, support, settings, and admin dashboard. |
| **Backend API** | Go 1.26.8 with Gin web framework and GORM ORM. 71 declared routes with middleware for JWT authentication, RBAC, rate limiting, and recovery. |
| **Database** | PostgreSQL 15 as production engine. Shared GORM auto-migration list enforcing restrictive foreign keys and nonnegative arithmetic check constraints. |
| **Economy Engine** | Double-entry wallet ledger (`WalletLedger`) and six-month FIFO coin batch tracker (`CoinBatch`). Locked transactional services (`CreditWalletTx`, `DebitWalletTx`) enforce strict point accounting invariants. |
| **Realtime** | WebSocket hub with thread-safe client writers, private target filtering, ongoing token checks, and disconnect cleanup. |
| **Infrastructure** | Multi-stage Dockerfile, Docker Compose stack with PostgreSQL health checks (`pg_isready`), and Vercel static deployment header configurations. |

---

## 4. Verification Evidence Log

```text
1. Go Compilation & Tests:
   go fmt ./...         -> PASS
   go vet ./...         -> PASS
   go build ./...       -> PASS
   go test ./...        -> PASS (ok profhit-backend/tests 16.048s)
   go mod verify        -> PASS (all modules verified)

2. Vulnerability & Race Audits:
   govulncheck ./...    -> PASS (0 reachable vulnerabilities)
   go test -race ./...  -> PASS (ok profhit-backend/tests 11.699s, 0 data races)

3. PostgreSQL Concurrency & Invariants:
   TestPostgresWalletConcurrency            -> PASS (1.98s)
   TestPostgresPaymentSettlementConcurrency -> PASS (1.15s)
   TestPostgresRedemptionStockConcurrency   -> PASS (1.64s)
   TestPostgresMarketResolutionConcurrency  -> PASS (1.52s)

4. Frontend & Formatting Suite:
   node scripts/check-frontend.mjs         -> PASS (25 HTML pages, 21 script checks)
   git -c core.safecrlf=false diff --check -> PASS (0 formatting/whitespace errors)

5. Container & Disaster Recovery Drills:
   docker build -t profhit-backend:test .  -> PASS
   docker compose up -d                    -> PASS (PostgreSQL 15 healthy, API 200 OK)
   pg_dump -> DROP DB -> CREATE DB -> psql -> PASS (100% data & schema integrity restored)
```

---

## 5. Deployment Operator Action Items for Final Production Certification (GO Status)

To transition PROPHIT from **CONDITIONAL GO** to **GO**:

1. **Credential Rotation**:
   * Generate a new 32+ character string for `JWT_SECRET` (`openssl rand -hex 32`).
   * Update PostgreSQL password on Neon/host and set `DATABASE_URL` / `DB_PASSWORD`.
   * Revoke and reissue `NEWS_API_KEY`, `GEMINI_API_KEY`, and Gmail App Password (`SMTP_PASSWORD`).
2. **Provider Staging Sandbox Credentials**:
   * Supply `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, and `RAZORPAY_WEBHOOK_SECRET` in environment variables.
   * Supply `HYPERVERGE_API_KEY`, `HYPERVERGE_API_SECRET`, `HYPERVERGE_WORKFLOW_ID`, and `HYPERVERGE_WEBHOOK_SECRET` in environment variables.
   * Supply `GOOGLE_CLIENT_ID` in environment variables.
3. **Staging Smoke Test**:
   * Deploy commit `785d7a0` to staging environment.
   * Verify live payment capture, KYC session start, email reset delivery, and Google sign-in.
4. **Data Rehearsal (If migrating existing database)**:
   * Dump production database, restore to disposable PostgreSQL instance, run `config.Migrate(db)`, and verify zero constraint or ledger balance errors.

---

## 6. Git Tracking Status

* **Branch**: `main`
* **Release Stabilization Baseline**: `785d7a0` (`fix: complete production release stabilization`)
* **HEAD before this uncommitted authentication update**: `08bcf6b` (`docs: update production audit report with Section 11 release gate matrix`)
* **Remote**: `https://github.com/SoorejS/profhit` (pushed to origin/main)
* **Working Tree**: Authentication source, tests and documentation changed; final status and secret scan are recorded after verification. No credentials were added.

---

## 7. SMTP, TOTP and Google authentication certification — 2 October 2026

| Capability | Status | Empirical evidence |
| --- | --- | --- |
| SMTP configuration | BLOCKED | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME` and `SMTP_PASSWORD` were not configured in the execution environment. No value was printed or copied into the repository. |
| SMTP delivery | BLOCKED | No dedicated staging mailbox/App Password was supplied, so an actual message and reset link could not be delivered or opened. |
| SMTP/reset failure handling | PASS | Missing configuration returned HTTP 503 in the browser without console errors. Connection refusal and invalid-port tests returned bounded, non-secret errors. Reset-token tests passed for valid use, reuse rejection, expiry, malformed input and atomic replacement. |
| TOTP 2FA local regression | PASS | Disposable local account completed password-confirmed enrollment, QR/provisioning generation, invalid-code rejection, standards-generated current-code enable, password-only login denial, valid-code login, invalid disable denial and valid-code disable. Enable/disable rotated all prior sessions. Mobile login/settings remained 390 px wide with no overflow; browser console was empty. |
| TOTP 2FA external certification | BLOCKED | A real authenticator application did not scan the QR and produce the accepted code. The release brief requires that empirical step before the overall TOTP gate can pass. |
| Recovery codes | NOT IMPLEMENTED | The API and Settings UI explicitly report that recovery codes are unavailable; no substitute recovery mechanism was invented. |
| Google OAuth configuration | BLOCKED | `GOOGLE_CLIENT_ID` and an exact staging frontend origin were not configured. The login page accurately displayed Google Sign-In as unavailable. |
| Google browser handshake | BLOCKED | No real staging OAuth client/account was supplied, so new-user/existing-user browser consent and logout could not be certified against Google. |
| Google validation logic | PASS (local regression only) | Missing configuration, provider rejection/expired-token response, wrong audience, account reuse, forged/malformed 2FA and valid 2FA enforcement are covered with isolated HTTP transport tests. This is not represented as a real Google integration pass. |

Final authentication gate values:

```text
SMTP staging         = BLOCKED
TOTP 2FA             = BLOCKED
Google OAuth staging = BLOCKED
Historical credential rotation = BLOCKED
Final production status = NO-GO
```

Authentication-update command evidence:

```text
go fmt ./...                                      PASS
go vet ./...                                      PASS
go build ./...                                    PASS
go test ./...                                     PASS (tests 5.926s)
go mod verify                                     PASS (all modules verified)
govulncheck@latest ./...                          PASS (0 reachable/imported; 1 unused module-only)
node scripts/check-frontend.mjs                   PASS (25 HTML, 21 scripts)
git -c core.safecrlf=false diff --check           PASS
git ls-files sensitive extensions                 PASS (0 tracked files)
added-line credential/private-key scan            PASS (0 suspicious assignments)
go test -race ./...                               BLOCKED on host: CGO disabled
CGO_ENABLED=1 go test -race ./...                 BLOCKED: gcc not installed
Docker race-test fallback                         BLOCKED: Docker daemon not running
```

The prior baseline's Linux-container race result remains valid for that commit, but the current uncommitted authentication delta has not passed a race-enabled build. This is a remaining verification gate rather than an application test failure.

---

## 8. Plan B email/password authentication certification — 2 October 2026

The disposable local application at `http://127.0.0.1:5174` was tested with SMTP and Google OAuth unset. Email/password registration and login remained fully functional. This is a PASS for the Plan B primary path; it does not change the overall production NO-GO while password recovery delivery, live Google OAuth, credential rotation, and the other release gates remain blocked.

| Authentication capability | Status | Evidence |
| --- | --- | --- |
| Email/password signup | PASS | Browser User A and User B registrations returned authenticated sessions and opened Dashboard. Server tests cover valid signup, missing/invalid fields, case-insensitive duplicate email/username, invalid username, short/blank password, and injected role/points/KYC fields. |
| Email/password login | PASS | User A logged out and signed in using a differently cased email while Google and SMTP were unset. Wrong password and unknown account return the same 401 response; malformed/empty requests return 400. |
| Logout | PASS | Both browser users logged out. Regression tests confirm the server records revocation and rejects token reuse. |
| Password hashing | PASS | Registration stores a bcrypt cost-12 hash; regression coverage compares the hash to the submitted password and confirms plaintext is not stored or returned. |
| Password reset | BLOCKED | Token replacement, valid use, expiry, reuse rejection, malformed token rejection, password update, and prior-session rotation pass locally. The browser correctly showed `Password reset email is temporarily unavailable`; the email/link journey cannot finish without SMTP. |
| SMTP delivery | BLOCKED | No staging SMTP configuration or mailbox was supplied. The application returned HTTP 503 and did not claim that a message was sent. |
| TOTP enrollment | PASS (local) | Browser User B completed password-confirmed setup and enabled TOTP with a standards-generated current code. Ownership is derived from the authenticated session, and another user's 2FA row remained unchanged in regression coverage. |
| TOTP login enforcement | PASS (local) | Password-only login returned 401 `2fa_required`; the current six-digit code completed login and opened Dashboard. Wrong, expired, and malformed OTP cases pass regression tests. External authenticator-app certification remains a separate release gate. |
| Google Sign-In | BLOCKED / OPTIONAL | With `GOOGLE_CLIENT_ID` empty, login and registration showed `Google Sign-In is not configured. Use email to continue.` No dead Google button was rendered. Local provider-contract tests pass, but no live OAuth client was supplied. |
| Session revocation | PASS | Protected `/api/me` accepted a valid JWT, logout revoked it, and reuse returned 401. Token identity and role match the database user. |
| Suspended-user enforcement | PASS | Existing regression coverage rejects disabled/suspended accounts; current database status is checked for every protected request. |
| Rate limiting | PASS | Six repeated invalid login requests from one IP produced five 401 responses followed by HTTP 429. |

Browser evidence:

```text
User A: Register -> Dashboard -> Logout -> email/password Login -> Dashboard -> Logout  PASS
User B: Register -> TOTP enrollment -> Logout -> password-only 2FA challenge -> TOTP -> Dashboard  PASS
Password reset with SMTP unset -> HTTP 503 explicit unavailable state                         PASS
Google configuration unset -> unavailable/optional message, email forms remain usable         PASS
Fresh login page browser console warnings/errors                                                0
Auth API responses observed: register 201; login 200; password-only 2FA 401; logout 200;
                             forgot-password 503; auth/config 200
```

Plan B status:

```text
Email/password signup: PASS
Email/password login: PASS
2FA: PASS
Password reset: BLOCKED
SMTP: BLOCKED
Google Sign-In: BLOCKED
Final Plan B authentication status: PASS
```


## PDF reconciliation sprint — 2026-10-03

Current rules and release status are recorded in [PDF_Reconciliation_Report.md](PDF_Reconciliation_Report.md). Earlier observations are historical. This local sprint does not certify production readiness, providers, PostgreSQL, race testing, credential revocation or deployment.
