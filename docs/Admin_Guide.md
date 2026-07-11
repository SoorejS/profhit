# PROPHIT Admin Guide

The Admin Dashboard provides comprehensive tools for managing the PROPHIT ecosystem during the closed beta.

## Accessing the Admin Panel
1. Navigate to `/admin.html`.
2. Login with an account that has the `RoleAdmin` or `RoleSuperAdmin` role. (During seeding, the default admin is usually `admin@prophit.app` / `password123` depending on configuration).

## Key Workflows

### User Management
- **View Users**: The dashboard lists all registered users along with their wallet balance and tier status.
- **Adjust Wallet Balance**: Admins can issue promotional coins or penalize bad actors directly from the user table. This utilizes the `TxTypeAdminAdjustment` ledger entry type.

### Market Management
- **Create Market**: Markets (Prediction questions) can be created via API or directly from backend seed scripts. Future UI iterations will support direct creation.
- **Settle Market**: Once an event occurs, an Admin must settle the market with the correct outcome. The system will automatically trigger payout calculations, notifying winners via WebSocket, and updating their Wallet Ledgers.

### KYC & Verification
- **Approve KYC**: In Beta mode, users perform dummy KYC. Admins can view pending requests and manually approve or reject them. Approving KYC will unlock the `100% Profile` achievement (awarding 250 coins).

### Withdrawals
- **Process Withdrawals**: Users who reach the minimum threshold can request a withdrawal. Admins review pending requests in the dashboard. Once marked "Approved," the platform deducts the internal coins, but the actual real-world transfer must be done manually by the Admin (until Phase 2 Payment Gateways are implemented).
