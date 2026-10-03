# PROPHIT administration

Use role-protected accounts. Market reviewers may publish only after checking the title/category, exact PDF format/payout, measurable rule, approved source, units and future cutoff. Politics wording must remain neutral. AI/news proposes opportunities; it cannot authorize settlement.

## Markets, news and challenges

The Markets tab lists proposed drafts and active/closed markets. Review rules through the form dialog, save, then publish. A draft with activity or a linked challenge cannot be rewritten. News drafts preserve the original article URL and publication timestamp; missing result rules/source prevent publication. Existing markets with historical submissions require a separately reviewed reconciliation plan.

Create weekly challenges from the admin form on `challenges.html`, using a reviewed market with no predictions and cutoff within seven days. The server doubles its normal payout and records participation through the standard prediction page. Challenge results show actual participants and rewards; history remains visible after completion.

Resolve only Locked/Awaiting Resolution markets. Enter the measured result in the published format, review the payout action, then supply approved-source evidence URL and an observation time at/after cutoff and no later than now. Successful settlement computes winners and posts coins, ledger, notifications and challenge results transactionally. A repeated settlement cannot pay again. Do not use an AI guess or an unverifiable headline as evidence.

## Verification and vouchers

The KYC tab shows actual provider attempts; it does not offer manual verification shortcuts. Document identity alone cannot authorize redemption. Current annual phone OTP and email confirmation must also be established by a certified workflow. That contact adapter and its staging tests remain pending.

Voucher queue states are Pending, Processing, Fulfilled, Delivered and Failed. Start processing a Pending request. Supply an actual officially sourced voucher and supplier/invoice reference to fulfil a Processing request. The code is encrypted and excluded from API responses. Fulfilment grants the stored tier badge, not email-delivery success.

Send a Fulfilled voucher through the delivery action only when real TLS SMTP is configured. A persisted dispatch claim prevents duplicate sends. Delivered means SMTP acceptance; inspect staging inbox/provider logs to establish receipt. Failed or interrupted delivery requires operator reconciliation of SMTP logs and the original request before any reset/retry. There is no automated retry/reconciliation action yet; never force Delivered or insert invented voucher codes.

Only Pending requests may be rejected by the normal queue action. Refunds reuse original, still-valid coin allocations and do not extend expiry or reinstate expired coins. Historical requests lacking allocations fail closed for accounting review. The older catalogue/history remains a separate legacy workflow and needs consolidation before a unified fulfilment launch.

Expected voucher delivery is 24–48 hours after current verification and official sourcing. Supplier contracts/buffer inventory, overdue notifications and confirmed email delivery remain operational gates. Platinum priority support and Diamond ad placement are pending; no functioning benefit should be promised until implemented.

## Moderation and release

User search, suspension/ban controls and report review retain hierarchy and ownership checks. Administrators cannot ban themselves or protected accounts. Bans revoke old sessions; unban permits a new login without restoring previous tokens. Automatic device/bot detection is not implemented; do not collect invasive fingerprints as a shortcut.

Follow the migration runbook in [Deployment_Guide.md](Deployment_Guide.md) and the gate status in [PDF_Reconciliation_Report.md](PDF_Reconciliation_Report.md). No local audit authorizes a commit, push, credential reuse, production-data change or deployment.
