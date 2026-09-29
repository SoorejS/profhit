# PROPHIT administration

Sign in as an authorized admin or super-admin and open /admin.html. Never use local seeded credentials publicly. Production does not seed administrators; provision the first administrator through a controlled database administration process.

- Markets: review proposals and active markets; resolve Locked or Awaiting Resolution markets using an exact declared option. Fixed payouts and statistics update in one transaction. Repeated resolution is rejected. Draft creation and state transitions are available through authorized APIs.
- KYC: inspect provider attempts and failures. This screen does not manually forge approvals. Signed HyperVerge callbacks update identity.
- Withdrawals: coins are deducted at request time. Rejection refunds once; approval does not debit again. Actual voucher fulfillment remains an operator responsibility. Catalog requests use the separate /api/admin/redemptions API.
- Analytics: totals and health come from /api/admin/stats and /api/health.
- Moderation: search users, ban/unban eligible accounts, and review reports. Self-ban and super-admin bans are rejected. Only super-admins can assign roles.

There is no public wallet-adjustment endpoint, Grafana integration, advertiser portal, or sponsored-prediction workflow. Roadmap labels do not indicate completed functionality.
