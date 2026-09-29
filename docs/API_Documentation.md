# PROPHIT API contract

Source of truth: backend/routes/routes.go. JSON errors use an error field. Supply Authorization: Bearer <JWT> on protected routes. Invalid/revoked/expired sessions return 401; insufficient roles return 403. The body limit is 1 MiB. Mutations must never be retried blindly unless their documented operation is idempotent.

## Routes

| Method | Path | Access |
| -- | -- | -- |
| POST | /api/auth/register | Public; auth rate limit |
| POST | /api/auth/login | Public; auth rate limit |
| POST | /api/auth/google | Public; auth rate limit |
| POST | /api/auth/forgot-password | Public; auth rate limit |
| POST | /api/auth/reset-password | Public; auth rate limit |
| GET | /api/health | Public; see notes |
| GET | /api/auth/config | Public; see notes |
| POST | /api/webhooks/hyperverge | Public; see notes |
| POST | /api/webhooks/razorpay | Public; see notes |
| POST | /api/simulator/sign-webhook | Public; see notes |
| GET | /api/news | Public; see notes |
| GET | /api/markets | Public; see notes |
| GET | /api/markets/:id | Public; see notes |
| GET | /api/markets/:id/comments | Public; see notes |
| GET | /api/leaderboard | Public; see notes |
| GET | /api/leaderboard/legacy | Public; see notes |
| GET | /api/leaderboard/streak | Public; see notes |
| GET | /api/leaderboard/winrate | Public; see notes |
| GET | /api/activity | Public; see notes |
| GET | /api/users/:id | Public; see notes |
| GET | /api/ws | Public; see notes |
| POST | /api/auth/logout | Authenticated user |
| GET | /api/me | Authenticated user |
| GET | /api/me/achievements | Authenticated user |
| GET | /api/notifications | Authenticated user |
| POST | /api/notifications/read | Authenticated user |
| POST | /api/kyc/start | Authenticated user |
| GET | /api/kyc/status | Authenticated user |
| POST | /api/me/daily-login | Authenticated user |
| GET | /api/me/streak | Authenticated user |
| GET | /api/wallet/history | Authenticated user |
| GET | /api/wallet/transaction/:id | Authenticated user |
| GET | /api/referrals/analytics | Authenticated user |
| POST | /api/payments/order | Authenticated user |
| POST | /api/payments/verify | Authenticated user |
| POST | /api/payments/redeem | Authenticated user |
| POST | /api/predictions | Authenticated user |
| GET | /api/predictions | Authenticated user |
| GET | /api/portfolio | Authenticated user |
| POST | /api/markets/propose | Authenticated user |
| POST | /api/markets/:id/comments | Authenticated user |
| POST | /api/reports | Authenticated user |
| GET | /api/me/reports | Authenticated user |
| GET | /api/rewards | Authenticated user |
| POST | /api/rewards/redeem | Authenticated user |
| GET | /api/me/redemptions | Authenticated user |
| POST | /api/markets | Admin, super-admin, content creator |
| GET | /api/markets/proposed | Admin, super-admin, content creator |
| POST | /api/markets/:id/approve | Admin, super-admin, content creator |
| PUT | /api/markets/:id/transition | Admin, super-admin, content creator |
| DELETE | /api/markets/:id | Admin, super-admin, content creator |
| POST | /api/markets/:id/resolve | Admin, super-admin |
| GET | /api/admin/redemptions | Admin, super-admin |
| PUT | /api/admin/redemptions/:id | Admin, super-admin |
| GET | /api/admin/stats | Endpoint-specific role; see notes |
| GET | /api/admin/users | Endpoint-specific role; see notes |
| POST | /api/admin/users/:id/ban | Endpoint-specific role; see notes |
| POST | /api/admin/users/:id/unban | Endpoint-specific role; see notes |
| PUT | /api/admin/users/:id/role | Endpoint-specific role; see notes |
| GET | /api/admin/audit-logs | Endpoint-specific role; see notes |
| GET | /api/admin/kyc | Endpoint-specific role; see notes |
| GET | /api/admin/kyc/:id | Endpoint-specific role; see notes |
| GET | /api/admin/withdrawals | Endpoint-specific role; see notes |
| POST | /api/admin/withdrawals/:id/approve | Endpoint-specific role; see notes |
| POST | /api/admin/withdrawals/:id/reject | Endpoint-specific role; see notes |
| GET | /api/admin/reports | Endpoint-specific role; see notes |
| POST | /api/admin/reports/:id/resolve | Endpoint-specific role; see notes |

## Access details

WebSockets require a token query parameter and a permitted Origin; do not log their query strings at a proxy. The simulator is super-admin-only and exists only in non-release BETA_MODE. Leaderboard accepts an optional bearer token to include the current user's rank. User profiles deliberately return only public profile fields. Wallet, KYC status, predictions, portfolio, referrals, achievements, notifications, and redemption history use the authenticated identity, never a caller-supplied user ID.

Admin stats/users/ban/unban permit admin, super-admin, and IT support (with account hierarchy checks for bans). Role changes and audit logs require super-admin. Admin KYC, withdrawals, and reports require admin or super-admin.

## Primary requests and responses

- POST /auth/register: username (3–64 characters), email, password (8–72), optional referral_code and demographic fields. Returns 201 with token and user.
- POST /auth/login: email, password, optional two_factor_code. Returns token and user.
- GET /auth/config: google_client_id, empty when unconfigured; contains no secrets.
- POST /auth/forgot-password: email. Uses a non-enumerating response and queues email delivery. POST /auth/reset-password: token and new_password. Token is one-time; password reset invalidates old sessions.
- GET /me: current profile including points, tier, role, and KYC status. GET /me/achievements: array of earned title/description/reward rows.
- GET /markets: array, default active statuses; category/status/sort filters and limit (1–100, default 50)/offset. Sort is trending, newest, or ending_soon. Drafts and proposals are not public.
- POST /markets or /markets/propose: title, category, description, options as a JSON-array string, difficulty, payout, lock_time (RFC3339) or end_date, optional start_time/resolution_time/source/visibility. Server validates categories, options, times, and difficulty bounds. User proposals always become Proposed.
- PUT /markets/:id/transition: status. POST /markets/:id/resolve: winner (or outcome). Resolution only from Locked/Awaiting Resolution; repeated resolution is rejected.
- POST /predictions: market_id, choice, amount (integer 10–1000000). Fixed potential payout is taken from the server market. Duplicate/day-limit/cutoff violations do not debit the wallet.
- GET /portfolio: array with market_id, market, market_status, choice, potential_payout, is_correct (null until settled). An archived market can still have a settled outcome.
- GET /wallet/history: array of credit/debit/balance ledger rows, optional type filter. GET /wallet/transaction/:id enforces ownership.
- POST /payments/order: amount (whole INR 10–100000), returns order_id, amount in paise, currency, and public key. POST /payments/verify: razorpay_order_id, razorpay_payment_id, razorpay_signature. A legacy points field is ignored. Settlement is idempotent for the same captured payment.
- POST /payments/redeem: tier (Bronze, Silver, Gold, Platinum, Diamond). Creates a pending voucher request and debits coins atomically.
- POST /rewards/redeem: reward_item_id. PUT /admin/redemptions/:id: status, voucher_code, admin_remarks; completion requires a voucher, rejection restores stock and refunds once.
- POST /kyc/start: no body required; returns verification_url/session_id or an explicit provider/pending error. GET /kyc/status: status/reason. Only a signed provider callback may approve identity.
- POST /me/daily-login: UTC-day idempotent check-in; returns coins_earned/current_streak/new_balance.
- GET /leaderboard: data/meta/current_user; sort=points|streak|winrate, page, limit up to 100, search.
- GET /notifications: array with limit/offset. POST /notifications/read marks only the caller's notifications read.
- POST /markets/:id/comments: content, nonblank and at most 2000 characters. POST /reports: target_type (User/Market/Comment), target_id, reason, description.

## Timing and deployment assumptions

Referral rewards are due after 48 hours and processed hourly. Coin reminders are persisted once within 30 days of expiry. User.Points matches the ledger and all remaining batches; expired batches await the hourly expiry debit, but are excluded from spending immediately. Dates and daily prediction/check-in limits use UTC.

The rate limiter is per-process/per-IP. Use shared edge rate limiting when deploying multiple replicas. Auth uses 5 requests per 5 minutes; prediction/proposal/comment/report mutations use 30/minute; payment/KYC/redemption mutations share 5/minute. Frontend and API are separate deployments.
