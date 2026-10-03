# PROPHIT API contract

Source of truth: `backend/routes/routes.go`. Protected requests use `Authorization: Bearer <JWT>`. JSON errors contain `error`. Invalid, revoked or expired sessions return 401; insufficient roles return 403. Bodies are limited to 1 MiB. Do not blindly retry non-idempotent mutations.

## Authentication

- `POST /api/auth/register`: username (3–64 letters, numbers or underscores), email, password (8–72), optional referral_code and demographic fields. Username/email are trimmed, normalized lowercase and unique case-insensitively. Role, coins and verification are server-controlled. Returns 201 with token/user and grants the separate one-time 100 welcome coins.
- `POST /api/auth/login`: email, password, optional two_factor_code. Returns token/user; a 2FA-enabled account receives `2fa_required` until a valid current code is supplied. Successful login claims the separate UTC-day +10 reward idempotently.
- `POST /api/auth/google`: credential and optional two_factor_code. Existing/new Google accounts use the same account and 2FA controls. `GET /api/auth/config` exposes only the public google_client_id; empty means unavailable. External staging certification remains required.
- `POST /api/auth/forgot-password`: email. Returns 503 when SMTP is unconfigured; otherwise uses a non-enumerating response and queues delivery. `POST /api/auth/reset-password`: token/new_password. Reset tokens are expiring, one-time and replacement-safe; successful reset revokes old sessions.
- `POST /api/auth/logout`: authenticated, revokes the current JWT. Banning an account increments token_version; unbanning cannot revive its previous sessions.
- `GET /api/me/2fa`: enabled, setup_pending, recovery_codes_enabled. `POST /api/me/2fa/setup`: current_password; returns a no-store provisioning URI, QR data URL and manual secret without enabling 2FA. `POST /api/me/2fa/enable`: current code; enables and rotates sessions, returning a replacement token. `POST /api/me/2fa/disable`: current code; clears the secret, rotates sessions and returns a replacement token. Recovery codes are not implemented.

## Profile and rewards

- `GET /api/me`: points is currently spendable coins; ledger_balance is the cached historical ledger total, including expiry awaiting processing. Also includes full_name, email, phone, city, country, interests, tier, role, KYC status, earned badges and pending benefit statuses.
- `PUT /api/me/profile`: full_name, phone, city, country, interests. Fields must be nonblank; phone accepts 7–15 digits with an optional leading `+`. Email comes from the authenticated account. Complete profiles earn exactly 30 once transactionally. KYC and 2FA do not define profile completion.
- `GET /api/me/stats`: total_predictions, settled_predictions, won_predictions, win_rate among settled predictions and pending_potential across the complete history; caller identity only.
- `GET /api/me/achievements`: earned title/description/reward rows. Current definitions are corrected without rewriting historical wallet awards.
- `POST /api/me/daily-login`: +10 once per UTC day. Returns already_checked_in, coins_earned, new_balance. Separate from prediction streaks.
- `GET /api/me/streak`: consecutive prediction days, longest/total days and next milestone. +25 on day 3 and +75 on day 7; repeated same-day predictions do not advance days. There is no invented continuous repeat-cycle reward.
- `GET /api/referrals/analytics`: caller's referral relationships and pending/paid events. Only the first valid prediction reserves a 50-coin reward, due after 48 hours; maximum 20 bonuses. Signup/KYC/deposit referral awards are disabled.

## Markets and predictions

`GET /api/markets` is public: array with category/status filters, sort=trending|newest|ending_soon, limit (default 50, max 100), offset. Defaults to active statuses; Draft/Proposed markets are not public. `GET /api/markets/:id` is public for published markets and enforces existing draft visibility rules. Public `GET /api/markets/:id/comments` uses pagination.

`POST /api/markets` permits admin, super-admin and content creator. `POST /api/markets/propose` requires an authenticated user and always creates Proposed. Supply title, description, category, difficulty, options as a JSON-array string, prediction_type, measurable resolution_rule, approved HTTPS resolution_source, range_width where applicable, future lock_time (RFC3339), and optional start_time/resolution_time/visibility. The server assigns the exact PDF format/payout and normalizes dates to UTC.

| Category | Easy | Medium | Hard |
| --- | --- | --- | --- |
| Weather | binary / 20 | range / 50 | exact / 120 |
| Sports | winner / 25 | range / 60 | score / 200 |
| Politics | winner / 30 | margin / 80 | seat_count / 250 |
| Entertainment | winner / 25 | top3 / 70 | range / 150 |
| Financial Markets | direction / 20 | percent_range / 80 | closest_price / 300 |
| Wild Card | binary / 40 | multi_choice / 100 | closest_text / 400 (measurable number) |

`POST /api/predictions`: market_id and choice. Omit amount or supply zero; every nonzero amount returns 400. Entry is free, including at zero coin balance. Potential reward is chosen by the server and captured at submission (2× for eligible challenge participation). One submission per market and one prediction per category per UTC day. Invalid, duplicate or late submissions cannot debit coins. Historical markets without reviewed typed rules return 409.

Choice is a declared option, a number, `[min,max]`, a score total or `[home,away]` as specified by the rule, or a JSON array of three unique declared nominees. Range width is bounded by the published rule. Margin tolerance is five percentage points; financial percentage-change tolerance is ±1 percentage point. Closest numeric ties share the fixed reward. Subjective free-text settlement is not supported.

`GET /api/predictions` requires authentication and paginates raw submissions. `GET /api/portfolio`: {items,page,page_size,total,total_pages}, default 20/max 100. Items include market_id, market, market_status, choice, potential_payout and is_correct (null until settlement). Archived settled results remain history.

`GET /api/markets/proposed`, `PUT /api/markets/:id/rules`, `POST /api/markets/:id/approve`, `PUT /api/markets/:id/transition` and `DELETE /api/markets/:id` permit admin/super-admin/content creator. Rule editing requires Draft/Proposed, no predictions and no challenge. Publication requires reviewed typed rules/source. Transition takes status; existing lifecycle validation applies. Deletion preserves accounting restrictions.

`POST /api/markets/:id/resolve` requires admin/super-admin: winner (or outcome), evidence_url, observed_at (RFC3339). Requires Locked/Awaiting Resolution, approved evidence on the configured source hostname and an observation at/after cutoff and no later than now. Typed winner calculation, payout snapshots, ledger, challenge results and notifications settle in one transaction. Evidence is persisted. Repeated resolution is rejected.

## Wallet, KYC and vouchers

- `GET /api/wallet/history`: paginated credit/debit/balance ledger rows with optional type filter. `GET /api/wallet/transaction/:id`: ownership enforced.
- `GET /api/wallet/batches`: batches, spendable, awaiting_expiry_processing. Six calendar months per earned batch with month-end clamping. Expired coins cannot be spent. FIFO debits and expiry ledger processing are idempotent.
- `POST /api/payments/order`, `POST /api/payments/verify`, `POST /api/webhooks/razorpay`: 410 Gone. No provider request or new purchase credit. Historical PaymentTransaction/WalletLedger rows remain for separate reconciliation.
- `POST /api/kyc/start`: provider URL/session or explicit error; expired annual verification may start again. `GET /api/kyc/status`: status, expires_at, redemption_eligible, phone_verified, email_confirmed. Signed `POST /api/webhooks/hyperverge` verifies identity only. Real phone OTP/email confirmation mapping is still pending; there is no client-controlled verification shortcut.
- `POST /api/payments/redeem`: tier. Bronze 500/₹50; Silver 1200/₹150; Gold 2500/₹350; Platinum 5000/₹800; Diamond 10000/₹2000. Requires current annual identity, phone OTP and email confirmation. Atomically creates Pending, consumes valid coins FIFO and records original-batch allocations.
- `GET /api/wallet/vouchers`: caller's actual tier voucher requests without codes/ciphertext.
- `GET /api/admin/withdrawals`: admin/super-admin, paginated actual states. `POST /api/admin/withdrawals/:id/approve`: Pending → Processing. `/fulfill`: voucher_code/source_reference; requires Processing and configured AES-GCM key, stores the actual sourced code encrypted and grants the earned badge. `/deliver`: requires Fulfilled and an unused persisted dispatch claim. Actual TLS SMTP acceptance permits Delivered; failure records Failed. Delivered means SMTP acceptance, not verified inbox receipt. Uncertain dispatches require operator reconciliation before retry; no automatic reset endpoint exists. Pending `/reject` refunds only original unexpired allocations once, never extending expiry.
- Platinum priority support and Diamond ad placement remain pending; Silver–Diamond badges are stored on fulfilment. Gold recognition uses the actual tier on the leaderboard.
- Separate legacy catalogue: `GET /api/rewards`, `POST /api/rewards/redeem`, `GET /api/me/redemptions`; current verification gate applies. `GET /api/admin/redemptions`, `PUT /api/admin/redemptions/:id` are admin/super-admin. Historical catalogue completion/rejection remains separate from the five-tier SMTP lifecycle; records lacking original allocations require reconciliation before refunds.

## Weekly challenges, news and live events

`POST /api/admin/challenges`: admin/super-admin, market_id of a reviewed Live/Scheduled market with no predictions and future cutoff within seven days. Returns 201; duplicate/ineligible links return 400. Public `GET /api/challenges` lists lifecycle/history; `GET /api/challenges/:id` returns {challenge,leaderboard} with username, score, reward_won. Participation and 2× reward snapshots commit with the prediction. Active becomes Closed at cutoff, then Completed on settlement.

Public `GET /api/news` reuses the existing provider/cache. Fresh articles (at most 24 hours old) become deduplicated editorial drafts with original URL/timestamp. A reviewer must supply measurable rules and an approved result source before publication. Unconfigured or failed news is explicitly unavailable; no fabricated headlines or AI settlement.

`GET /api/ws` requires a valid token query parameter and permitted Origin. Never log query strings. Committed mutations emit prediction_count_changed, market_activity_changed, market_locked, market_resolved, notification_created, leaderboard_updated and wallet_updated. Clients refetch authoritative HTTP data with bounded reconnect; revocation also applies to long-lived sessions.

## Other routes and access

Public: `GET /api/health`, `/api/activity`, `/api/users/:id`, `/api/leaderboard`, `/api/leaderboard/legacy`, `/api/leaderboard/streak`, `/api/leaderboard/winrate`. Unified leaderboard accepts an optional bearer token for current_user and sort=points|streak|winrate, page/limit/search. Public profiles expose only public fields.

Authenticated: `GET /api/notifications`, `POST /api/notifications/read`, `POST /api/markets/:id/comments` (nonblank content up to 2000), `POST /api/reports` (target_type/target_id/reason/description), `GET /api/me/reports`. Every personal route uses the authenticated identity.

Admin/super-admin/IT support: `GET /api/admin/stats`, `/api/admin/users`, `POST /api/admin/users/:id/ban`, `/unban`, with account hierarchy/self-ban protections. Super-admin only: `PUT /api/admin/users/:id/role`, `GET /api/admin/audit-logs`. Admin/super-admin: `GET /api/admin/kyc`, `/api/admin/kyc/:id`, `/api/admin/reports`, `POST /api/admin/reports/:id/resolve`. `POST /api/simulator/sign-webhook` is super-admin-only and available only in non-release BETA_MODE.

## Timing and deployment

Referral payouts, expiry and reminders run hourly; market transitions run every minute. Coins awaiting expiry are excluded from spending immediately. New dates/query cutoffs use UTC. Existing non-UTC text timestamps need explicit reconciliation.

Per-process/per-IP limits: authentication 5/5 minutes, prediction/proposal/comment/report mutations 30/minute, payment/KYC/redemption mutations 5/minute, security mutations 10/5 minutes. Multiple replicas need shared edge enforcement. Frontend/API deploy separately. Local tests do not certify provider delivery, PostgreSQL locks or production data.
