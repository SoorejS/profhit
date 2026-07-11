# PROPHIT API Documentation

The backend is built in Go (Gin Framework). All routes generally fall under `/api`. 
Most routes except `/api/auth/*` and `/api/health` require a valid JWT passed in the `Authorization: Bearer <token>` header.

## Public Routes

- `GET /api/health`: Returns HTTP 200 `{"status": "ok", "message": "PROPHIT Backend is running"}`. Used for infrastructure health checks.
- `GET /api/markets`: Returns all active markets.
- `GET /api/markets/:id`: Returns details of a specific market.
- `GET /api/markets/:id/comments`: Returns comments for a market.

## Authentication

- `POST /api/auth/register`: Expects `{"username", "email", "password", "referral_code"}`.
- `POST /api/auth/login`: Expects `{"username", "password"}`. Returns `{"token"}`.

## User & Wallet (Requires JWT)

- `GET /api/users/me`: Returns the current authenticated user's profile.
- `GET /api/wallet/ledger`: Returns a paginated list of double-entry wallet transactions (Credits, Debits).
- `GET /api/wallet/balance`: Returns the current computed balance.

## Predictions (Requires JWT)

- `POST /api/markets/:id/predict`: Places a prediction. Expects `{"outcome", "amount"}`. Deducts from wallet.
- `GET /api/users/me/predictions`: Returns user's active/past predictions.

## Admin Operations (Requires RoleAdmin)

- `POST /api/admin/markets/settle`: Settles a market and distributes winnings.
- `POST /api/admin/users/:id/adjust-wallet`: Adjusts user balance manually.
- `GET /api/admin/kyc-queue`: Lists pending KYC requests.

## WebSocket (Requires JWT via Query Parameter)

- `ws://localhost:8080/api/ws?token=<JWT>`: Establishes a real-time connection. Listens for events like `achievement_unlocked` or `market_settled`.
