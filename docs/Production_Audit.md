# PROPHIT stabilization audit — 29 September 2026

## Executive summary

Substantial code fixes and regression tests are implemented in the existing Go/Gin/GORM backend and static HTML/CSS/JavaScript frontend. The original architecture is preserved. The checkout already contained a broad stabilization change set when this audit began; those changes were reviewed and extended rather than discarded. This report describes the combined working tree, not an assertion that every change originated in this session.

**Release status: not yet approved for production. Commit and push are on hold.** The authenticated local browser matrix is complete and its reproducible defects are fixed. PostgreSQL/Docker integration could not run without a running Docker engine, and real payment/KYC/email integrations have not been certified in staging. Previously tracked credentials require external rotation. Passing SQLite tests does not establish PostgreSQL locking correctness under production load.

## System map

| Layer | Actual implementation and boundaries |
|---|---|
| Frontend | 25 HTML pages; CSS tokens/layout/components; native ES modules; no npm application manifest or frontend framework. Page modules cover auth, dashboard, market, portfolio, wallet/KYC, rewards, profile, leaderboard, notifications, proposal, support, settings and admin. Shared sidebar/topbar/toasts and API client. |
| API | 67 router declarations, including development-only simulator registration. Public discovery/auth/webhooks; authenticated self-service; content/admin/super-admin role groups. Full route inventory and contracts are in API_Documentation.md. |
| Startup | main validates environment, opens database, runs shared migrations, optionally seeds development data, backfills user statistics, starts cron and HTTP server. |
| Data | GORM models: User, Market, PredictionSubmission, WalletLedger, CoinBatch, ReferralEvent, PaymentTransaction, UserStreak, HyperVergeKYC, Achievement/UserAchievement, RewardItem/Redemption, Withdrawal, Report, Comment, AuditLog, PasswordResetToken, RevokedToken, Notification, and challenge models. PostgreSQL is production storage; SQLite is development/test only. |
| Economy | Wallet services serialize users, write immutable ledger effects and credit/consume coin batches in a transaction. Prediction, payment, redemption, achievements, expiry and referrals use these services. |
| Market lifecycle | Draft/Proposed editorial states; Scheduled/Open/Live accepting states subject to start/cutoff; Locked/Awaiting Resolution; Resolved/Archived. Payouts and choices are server validated. |
| External systems | Razorpay order/capture verification and signed webhooks; HyperVerge sessions and signed callbacks; Google ID token validation; SMTP password reset; news API for editorial wildcard drafts. Credentials are environment configuration. |
| Background work | Minute market transitions; hourly expiry/reminders/referrals/session cleanup; daily wildcard draft. Conditional claims and unique keys protect repeat processing. No durable external job broker. |
| Realtime | One authenticated WebSocket hub, bounded global/client queues, one writer per connection, private target filtering, ongoing token checks and disconnect cleanup. Current frontend modules do not subscribe to these events. |
| Infrastructure | Root and backend Docker build contexts, Compose PostgreSQL persistence/health checks, static Vercel deployment exclusions. No checked-in CI pipeline was found. |

## Bugs found and fixes

Severity identifies the consequence of the original defect, not a promise that the entire area is certified.

| ID | Severity | Area | Root cause | Fix / evidence |
|---|---|---|---|---|
| S01 | P0 | Secrets | .env, database and compiled binaries were tracked | Removed those paths from the Git index while preserving local files; tightened Git/Docker/Vercel exclusions. Historical credentials still require rotation. |
| S02 | P0 | Payments | Client-supplied points could determine credit; webhook only acknowledged events | Persist expected amount/owner, validate captured provider payment and HMAC, converge verification/webhook on locked atomic settlement, unique payment ID. Forged amount/replay tests. |
| S03 | P0 | Wallet | Obsolete GORM locking option and tolerance of missing valid batches | Real row-lock clauses, positive-amount/overflow checks, valid-batch FIFO and rollback. Concurrency and three-way balance tests. |
| S04 | P0 | Coin expiry | Generic FIFO debit could consume another batch during expiry | Lock account and exact expired batch; ledger debit and batch zeroing atomically, repeat calls harmless. |
| S05 | P0 | Auth/RBAC | In-memory revocation disappeared on restart; role claims could become stale | Persist hashed revocations, check current database role/status/version; reset revokes old sessions; random JWT IDs. Route/role/revocation tests. |
| S06 | P0 | KYC | Empty webhook secret and insufficient state/concurrency checks | Reject absent config, validate signatures/event states, lock updates, atomically record referral milestones; restrict simulator to development super-admin. |
| S07 | P1 | Migration | Production migration omitted PaymentTransaction | One shared production/test migration list including payments, sessions and notifications; repeated migration regression. |
| S08 | P1 | Referrals | Duplicate/cap races and payout retry inconsistencies | Unique milestones, serialized cap accounting, persistent 48-hour pending records, conditional transactional payout claim; delay/cap/self/duplicate/concurrency tests. |
| S09 | P1 | Predictions | Cutoff/daily limits could race or ignore real times | Market/account locks, UTC day interval, server window/choice/payout checks. Same-day, cutoff, duplicate and UTC-midnight boundary tests. |
| S10 | P1 | Resolution | Repeat/concurrent resolution could produce inconsistent outcomes | Locked legal state transition, ordered payout processing and transaction rollback; repeat-resolution test. |
| S11 | P1 | Redemptions | Refund/stock races and inconsistent lock order | Lock reward before wallet, one transition/refund/restock transaction; repeated rejection regression. |
| S12 | P1 | Achievements/daily reward | Separate claim and wallet operations could duplicate or lose rewards | Transactional unique achievement claim and reward; serialized daily claims. Require enabled 2FA, not merely an enrolled secret. |
| S13 | P1 | WebSocket | Unbounded asynchronous writes and stale authorization | Bounded single writers, disconnect cleanup and token validation on outbound messages/pings; two-client private isolation/revocation regression. |
| S14 | P1 | Dependency | Reachable x/text invalid-input infinite-loop advisory | Updated x/text and related patched networking/crypto dependencies, pinned Go 1.26.8; official scanner rerun. |
| S15 | P2 | Leaderboard | PostgreSQL-only cast broke SQLite; unresolved predictions/ties/rank eligibility disagreed | Portable resolved win-rate calculation, matching tie ordering and active-user rank filters; SQLite and tied-rank regressions. |
| S16 | P2 | Notifications | Expiry reminder was only a log and could repeat | Persist notification and reminder timestamp in one transaction with unique key; real notification API/UI. |
| S17 | P2 | Auth UX | Temporary suspension could issue an unusable session; Google UI used fixed config | Reject suspension on login; expose public Google client configuration; render explicit unavailable state and optional authenticator input. |
| S18 | P2 | Reset email | Plain reset-token storage, unbounded SMTP and misleading unavailable-email response | Hash newly issued tokens, serialize replacement, explicit 503 when SMTP absent, bounded TLS/STARTTLS connection, asynchronous failure logging. |
| S19 | P2 | Data integrity | Missing foreign keys/checks allowed malformed finance records | Restrictive relationships and nonnegative/ledger arithmetic/prediction/cost/inventory constraints; migration and invalid-record tests. |
| S20 | P2 | Moderation | Concurrent report decisions and privileged-target checks | Lock report before pending check, validate target privilege; live admin reporting actions. |
| S21 | P2 | Exposure | Draft comments and unlisted activity could escape public discovery boundaries | Reject draft comment reads/writes; exclude unlisted activity and suppress unlisted global comment/trade/lifecycle events. |
| S22 | P2 | Frontend contracts | Hardcoded admin/profile values, archived outcomes and duplicate /me calls | Actual stats/health/users/reports/achievements, outcome based on is_correct, share only in-flight /me calls. |
| S23 | P2 | Wallet UX | Pending KYC started duplicate sessions; payment failures lacked useful states | Status refresh for active verification, provider SDK/payment error handling and balance refresh; native accessible payment dialog. |
| S24 | P2 | XSS/navigation | Unsafe HTML values, URL schemes and interpolation | Escape dynamic text, reject unsafe URLs, encode query/path values, regression checks with malicious markup. |
| S25 | P2 | Deployment | Unsafe production defaults, missing exclusions and undocumented configuration | Production requires PostgreSQL, no demo seeding in release, URL-escaped DSN credentials, SSL mode config, timeouts, matching guides/build files. |
| S26 | P2 | Responsive UI | Landing action overflow at 320px; cramped grids/sidebar | Wrapping header/navigation, profile/settings layout, mobile sidebar close, bounded wrapping toast and retained daily rewards. Public-page width checks pass. |
| S27 | P3 | Accessibility/content | Click-only controls, missing labels, misleading roadmap/rule copy | Native buttons, labels/live toast, keyboard-friendly modal/menu controls; truthful roadmap and actual UTC/fixed-payout guidance. |
| S28 | P1 | Market integrity | Market detail rendered fabricated 55/45 odds, volume and participant totals | Removed invented figures; render the API's fixed payout and actual prediction count. Static regression rejects the old values. |
| S29 | P2 | Failure states | Market and profile pages could leave loading placeholders forever when required APIs failed | Render explicit unavailable states; profile loads independent sections with settled requests. Browser outage retest leaves no spinner. |
| S30 | P2 | Session UX | Logout retained the local bearer token if the revocation API was unreachable | Clear local authentication and redirect in a `finally` block after the best-effort server logout. |
| S31 | P2 | Admin UX/RBAC | An API outage redirected an authenticated admin to login; privileged self-actions appeared actionable | Distinguish transport failure from authorization, render an admin error state and disable protected super-admin moderation targets. |
| S32 | P2 | API status | Missing news configuration returned an internal-error response | Added a typed not-configured error and return 503; provider failures return 502. Controller regression verifies the 503 contract. |
| S33 | P2 | Browser/API efficiency | Admin market startup issued duplicate market and proposal requests | Route the initial hash through one tab loader. Final access log shows one GET per endpoint. |
| S34 | P2 | Accessibility | Settings, support, proposal and wallet controls lacked accessible names or label relationships | Added explicit labels and accessible names; desktop/mobile browser sweep found no visible unlabeled control. |
| S35 | P2 | Leaderboard contract | The current-user leaderboard object omitted the username | Populate the real username across ranking modes and assert it in regression coverage. |
| S36 | P3 | Console diagnostics | Expected handled HTTP responses were reported as JavaScript console errors | Preserve status on API errors and reserve console errors for transport/runtime failures; fresh success sweep is console-clean. |
| S37 | P3 | Seed content | Seeded NDA description contradicted its binary majority question/options | Corrected the development seed description. Existing databases require an intentional content migration if that text was already seeded. |

Dependency source: [official GO-2026-5970 advisory](https://pkg.go.dev/vuln/GO-2026-5970).

## Security review

Reviewed SQL construction/parameter binding, dynamic HTML and URL sinks, authentication storage, ownership checks, role boundaries, reset/session lifecycle, body size limits, CORS/trusted proxies, provider signatures and WebSocket routing. No server-side shell execution path was found in the application flow. Bearer-token authentication means ordinary cookie-CSRF assumptions do not directly apply; XSS protection remains essential because frontend tokens are stored in localStorage. No claim is made that code review proves absence of every exploit.

Tracked .env history contains non-placeholder DB_PASSWORD, JWT_SECRET, NEWS_API_KEY, GEMINI_API_KEY and SMTP_PASSWORD values. Values were not reproduced in this report. Rotate/revoke any that were used, invalidate affected sessions, and examine provider activity. Removing files from the current index cannot revoke secrets already present in history. History was not rewritten.

## Backend and database review

Reviewed controllers, middleware, models, transactional services, cron and WebSocket code, including the preexisting dirty changes. Added regression tests use the same migration function as production and enforce SQLite foreign keys. They verify User.Points = ledger credit-minus-debit = remaining batch balances after committed operations. Expired-but-not-yet-reconciled batches can remain in those persisted totals until expiry processing; spending excludes expired batches, so displayed total and immediately spendable balance can temporarily differ.

New constraints deliberately fail on invalid existing rows. Back up and rehearse migrations on a copy of the actual PostgreSQL database; this audit did not silently rewrite historical accounting or invent missing coin batches. Financial records use restrictive relations; existing soft-delete conventions remain. SQLite serializes test connections, so the concurrency tests check atomic application outcomes but cannot prove PostgreSQL row-lock behavior.

## Frontend and API review

All 25 HTML files and module references passed structural/syntax/local-link checks. Public pages tested at 320, 375, 390, 430, 768, 1024, 1280, 1440 and 1920 pixels: landing, login, register, forgot/reset password, privacy, terms, API docs, trading rules, 404, 500, maintenance and KYC simulator. No measured horizontal overflow remained in that matrix. Screenshots were inspected for the landing mobile correction. The current login page correctly displays Google sign-in unavailable when it is not configured.

Authenticated browser verification completed against the disposable SQLite development app at 1440 and 390 pixels for dashboard, markets, market detail, portfolio, wallet/KYC, rewards, profile, leaderboard, notifications, settings, support, proposals and every admin tab. The final 26 page/width combinations had no redirect, horizontal overflow, lingering loading indicator, visible unlabeled control or visible error alert. Mobile navigation open/close and the profile menu worked. A fresh successful-flow console contained no warnings or errors.

Success flows covered daily claim, a local points prediction, comments, portfolio and ledger updates, search/sort, leaderboard modes, proposal submission and admin queues. Empty states covered a separate seeded user and the relevant rewards, redemptions, notifications, portfolio and admin queues. Invalid-input checks covered prediction amount, duplicate/category limits, deposit minimum and proposal dates. No payment order, Razorpay flow, KYC session, real email or provider action was started. Session invalidation after a backend restart redirected to login; a regular user was denied the admin page; logout during an API outage still cleared the local session. During an intentional backend outage, every data page settled into an explicit failure state rather than retaining a spinner. Initial loading markup is present in source and both success/failure completion were verified; artificial network throttling was not added.

Final network evidence: authenticated application endpoints returned 200, CORS preflight returned 204, and the deliberately unconfigured `/api/news` returned 503 with a user-facing unavailable state. The admin market tab issued one GET `/api/markets` and one GET `/api/markets/proposed`. The only console messages during the outage exercise were expected transport failures of the exact form `[API Error] <endpoint>: TypeError: Failed to fetch`; the final fresh success sweep returned `[]`. The proposal approval button reached its native confirmation prompt, but the approval request itself was not sent because that browser surface could not accept the native dialog; the proposal remained pending and no irreversible admin action was performed.

Admin users/reports now use bounded queries; discovery and existing queues already accept bounds. Some legacy personal-history, catalog, comment and KYC list endpoints remain unbounded, with frontend statistics derived from full histories. Correcting those requires coordinating pagination and aggregate contracts and then exercising authenticated pages; silently truncating these responses would produce false totals or inaccessible records. This is remaining work, not a completed performance certification.

## Deployment findings

Go source and Docker builders use Go 1.26.8. Compose persists PostgreSQL and waits on health checks. DB_SSLMODE defaults to require; the local Compose database uses disable only on its internal network. Demo seeding and signing simulator are prohibited in release. Integration secrets are not bundled in images/static deployments. Database failure logs do not echo credentials. Updated deployment/admin/user/API guides describe actual behavior.

Docker CLI is installed but its engine was unavailable. No container build, PostgreSQL migration rehearsal, backup restore, multi-instance soak, provider staging certification or production deployment was performed. SMTP, Google, Razorpay and HyperVerge configuration must be supplied and tested by the deployment operator.

## Tests and results

Commands run from backend with GOCACHE set to an isolated temporary cache:

```text
go fmt ./...
go vet ./...
go build ./...
go test ./...
go mod verify
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Final verification after the authenticated-browser fixes: go fmt, go vet, go build and go test all passed; backend tests completed in 3.548 seconds. go mod verify passed with `all modules verified`. The final govulncheck exited 0: zero reachable vulnerabilities, zero vulnerable imported packages, and one module-only advisory in unused dependency code. That remaining advisory was reported without an available fixed version in the preceding verbose scan (GO-2026-5932); it is not represented as a reachable application finding. The scanner result is a point-in-time dependency analysis, not a proof of total application security.

```text
node scripts/check-frontend.mjs
PASS: 25 HTML pages, 21 script syntax checks, local links/modules,
HTML structure, duplicate IDs, XSS escaping and URL regressions.

git -c core.safecrlf=false diff --check
PASS after correcting documentation whitespace.

go test -race ./...
BLOCKED: race requires CGO; CGO_ENABLED=1 then failed because gcc is absent.
```

No npm audit was applicable because there is no npm dependency manifest. Provider tests use isolated fake HTTP transports to exercise verification/retry/failure logic; production code uses real providers. Regression coverage includes FIFO/expiry/rollback, concurrent debits/daily claims/referral payout, cap and delay, ownership and all protected-route anonymous denials, missing webhook configuration, payment amount/replay checks, database constraints, suspension, leaderboard portability/ties, UTC midnight, draft privacy and live WebSocket private-event isolation/revocation.

## Remaining issues and release gates

| Item | Severity | Why it remains | Beta blocker | Production blocker |
|---|---|---|---|---|
| Historical credentials | P0 if any were live | Revocation/rotation is an external operator action; removing tracking is insufficient | Yes with affected credentials | Yes |
| PostgreSQL/container validation | P1 validation gap | Docker engine unavailable; real row-lock/concurrent migration behavior unverified | Yes for money-bearing/shared beta | Yes |
| Real payment/KYC/email/Google staging | P1 validation gap | No authorized configured staging accounts/provider callbacks available | Yes for those enabled features | Yes for those enabled features |
| Existing accounting/migration rehearsal | P1 operational gate | Actual deployment data not supplied or changed | Yes if importing existing data | Yes |
| Legacy unbounded list contracts | P2 | Needs coordinated pagination plus aggregate/UI validation; not silently truncated | Small controlled beta only | Yes before scale |
| Race detector/soak | P2 validation gap | C compiler absent; no production-like concurrent soak | Not for isolated local evaluation | Required before production certification |
| Token storage and frontend CSP | P2 hardening limitation | localStorage tokens and inline scripts remain existing architecture; backend headers do not secure static-host documents | Requires accepted risk and XSS testing | Harden static hosting and review CSP before release |
| Roadmap features | P3 / explicitly unsupported | Some settings/challenges/integrations are future work | No if clearly labeled | No if clearly labeled |

## Git

Branch: main
Existing HEAD: 83fab1e (feat: Prioritize DATABASE_URL in ConnectDB for Neon PostgreSQL)
Stabilization commit: not created
Commit hash: not applicable
Push status: not pushed
Working tree: 94 changed tracked entries, 11 untracked entries (including new source/tests/docs/scripts), and 6 staged deletions at the last status check; local excluded files retained.
Remote: https://github.com/SoorejS/profhit

Commit/push are withheld by explicit instruction after the local browser audit. No force push or history rewrite occurred. After clearing the remaining external and production-like gates, rerun validation, review the complete diff including previously dirty files, create one clean stabilization commit, verify branch/remote/status and push main.
