# PDF reconciliation and local audit — 2026-10-03

The approved P0 economy changes and local typed prediction game are implemented and tested. This is **not a production-readiness certificate**. No commit, push, production data change, live payment, live verification or real email was performed. Provider workflows, PostgreSQL, race testing, historical-data transition and several PDF product features remain open.

Authority: `PredictionGamePlan(4).pdf`, pages 1–12, and the owner's explicit free-play/six-month/profile/streak/referral decisions. PDF legal statements are specification content, not independently verified legal conclusions. Earlier authentication work remains in the working tree.

The matrix below contains 57 individually scoped rows: 32 PASS locally, 13 PARTIAL and 12 NOT IMPLEMENTED. This is a requirement count, not a weighted completion percentage.

## PDF compliance matrix

PASS means implemented and verified locally for the stated scope. It does not close external staging or production gates. PARTIAL means there is working functionality with a named gap. NOT IMPLEMENTED means no functioning end-to-end implementation; placeholders do not count.

| Requirement | Status | Evidence / remaining limit |
| --- | --- | --- |
| Free participation, no purchased prediction coins or money stakes | PASS | Zero-balance submission succeeds; nonzero amount rejected; coin purchase/order/verify/webhook return 410; checkout removed. No coin entry fee was invented. |
| Coins not transferable between users | PASS | No transfer endpoint or UI; wallet identity/ownership enforced. |
| One-time 100 welcome coins (PDF pp.2,5) | PASS | Retained ledger-backed signup award; browser signup shows 100 plus distinct daily 10. |
| Name/email/phone/location/interests capture | PARTIAL | Settings captures all required completion fields; registration name entry and consent/legal controls need further work. |
| Six calendar months per earned batch | PASS | UTC calendar helper clamps month ends; credits and demo seeding share it; February/leap-year tests. |
| FIFO, independent expiry, no expired spend/reinstatement | PASS | Valid-batch debit; original allocation refunds never extend/revive expiry; economic invariant tests. |
| Expiry ledger, idempotent processing and valid remaining balance | PASS | Aged disposable 100-coin batch processed twice; one debit, remaining coins intact in browser. |
| 30-day reminder | PASS | Persistent per-batch claim/notification; two service calls produce one browser-visible reminder. Hourly scheduling delivers within the 30-day window, not at an exact second. |
| Safe old-expiry transition | PARTIAL | Read-only preview and explicit audited/idempotent apply CLI tested. Production-data review/application and PostgreSQL rehearsal not performed. |
| Complete profile = 30 once | PASS | Name, account email, valid phone, city, country, interests; concurrent claims and repeat browser save pay once; KYC/2FA not substitutes. |
| Daily login = 10 once per UTC day | PASS | Separate claim table/ledger logic; repeated logins/page loads do not mint more coins. |
| Prediction streak = consecutive UTC prediction days | PASS | Same-day/concurrent duplicates cannot add days; missed days reset. Browser shows actual day 1. |
| Day 3 = 25; day 7 = 75, no invented cycles | PASS | Eight-day repeated-submission regression yields exactly 100 streak coins plus distinct daily rewards. Multi-day milestones verified in service tests, not seven days of live browser use. |
| Referral relationship/self/duplicate prevention | PASS | Signup relationship only; duplicate/self blocked; persistent constraints and regression coverage. |
| First valid prediction referral = 50 | PASS | Other milestones disabled. Browser referred account predicts; owner sees pending 50, no immediate payout. |
| Referral delay 48h, cap 20, restart/idempotence/concurrency | PASS | Persistent pending row, locked conditional payout; 25 referred users reserve/pay only 20 under repeated concurrent jobs. Device-fraud evaluation is still limited below. |
| One prediction per category per UTC day, immutable submission/cutoff | PASS | Category/day checks and market locking; no edit flow; duplicate/concurrent/cutoff tests. |
| Six categories with three difficulty formats | PASS | All 18 server type/payout combinations tested. |
| Weather binary/range/exact: 20/50/120 | PASS | Typed validation; range browser invalid/valid checks; official-source allowlist. Live data adapter staging remains open. |
| Sports winner/range/score: 25/60/200 | PASS | Score total or pair as declared by rule; pair browser submission/settlement verified. |
| Politics winner/margin/seat count: 30/80/250 | PASS | Numeric margin tolerance disclosed as 5 percentage points; exact integer seats; ECI source gate. Editorial neutrality remains a review duty. |
| Entertainment winner/top3/collection range: 25/70/150 | PASS | Three unique nominees required, unordered matching; invalid two-pick/valid three-pick browser checks. |
| Finance direction/percentage/closest price: 20/80/300 | PASS | ±1 percentage point numeric guess and deterministic closest-price ties; browser price submission/settlement. |
| Wild Card binary/four-option/closest free text: 40/100/400 | PASS | Two/four options; free text limited to a measurable numerical answer. No subjective settlement. Browser four-option admin settlement succeeds. |
| New surprise Wild Card topic every day | PARTIAL | Fresh news creates review drafts; no guaranteed daily publication/automatic event extraction or staffed editorial cadence certified. |
| Prediction type/rule/source/deadline required | PASS | New publication and submission enforce reviewed rules; unreviewed legacy markets fail closed. |
| Trusted evidence and deterministic winner calculation | PASS | Approved HTTPS domains plus configured hostname, cutoff/observation checks, atomic payout snapshots, stored evidence; AI cannot settle. Source content itself is human-attested, not automatically verified. |
| Real source ingestion/fallback per category (PDF p.12) | PARTIAL | Allowlist and evidence workflow work; provider adapters, fallback correctness and outages need staging certification. |
| Weekly lifecycle, participation, cutoff, results/history, 2× ledger/notification | PASS | Linked market, persistent participant, Active→Closed→Completed, concurrent settlement test; browser two winners receive 100 instead of 50 and view results. |
| User proposals support typed formats | PASS | Normal user range proposal created, reviewed and published through protected API; server controls payout. |
| News reuses existing provider, timestamps, freshness/duplicates, review | PARTIAL | Real-provider/cache pipeline and editorial draft tests; browser shows unavailable news with no key. Live provider success and event extraction remain unverified. |
| Real multi-user WebSocket frontend | PASS | Existing infrastructure reused; second account changes counter 1→2 without reload; settlement/wallet/notification/leaderboard events refetch real state. Not a distributed scale/load certification. |
| Bronze 500/₹50, Silver 1200/₹150, Gold 2500/₹350, Platinum 5000/₹800, Diamond 10000/₹2000 | PASS | Exact thresholds/debits in five-tier regression; wallet exposes exact tiers and rejects unverified browser redemption. |
| Silver/Gold/Platinum/Diamond badges and Gold recognition | PASS | Fulfilment stores badge/tier; profile renders earned badges and leaderboard tier recognition. Positive tier tests use explicit test-only verification/sourcing fixtures, not real vouchers. |
| Platinum priority support | NOT IMPLEMENTED | Explicitly pending in UI; no priority operational queue. |
| Diamond free ad slot | NOT IMPLEMENTED | Explicitly pending; advertiser infrastructure deferred. |
| Mandatory annual redemption verification gate | PASS | Current identity/contact timestamps required; 12-month expiry and annual re-start tests; no manual/client verification shortcut. |
| Phone OTP + email confirmation end-to-end | NOT IMPLEMENTED | Contact workflow adapter/provider contract is pending. Identity-only HyperVerge approval cannot redeem. |
| Official voucher sourcing | PARTIAL | Protected actual-code/supplier-reference input and encrypted storage. Official Amazon supplier/API contract and provenance certification not verified. |
| Pending/Processing/Fulfilled/Delivered/Failed state, actual SMTP tied to delivery | PARTIAL | Protected transitions, encrypted code, persisted dispatch claim; absent SMTP gives Failed and blocks duplicate send. Real TLS SMTP acceptance/inbox delivery not exercised. |
| Voucher 24–48h operations, buffer, delay notification (PDF pp.7,12) | PARTIAL | Honest expectation and actual status UI; supplier buffer/SLA/overdue notifications and uncertain-dispatch admin reconciliation action still pending. |
| Legacy catalogue fulfilment unified with tier vouchers | PARTIAL | Catalogue history preserved and current KYC/refund gate enforced; its older Completed workflow is separate and needs operational consolidation. |
| Existing rate limits, suspension, locking, ownership | PASS | Preserved; ban now increments token version, so unban cannot restore old sessions; browser and regression test verify. |
| Privacy-appropriate device fingerprint/bot detection | NOT IMPLEMENTED | Rate limits/referral caps/manual suspension retained; automatic device/bot detection and consent policy need design/review. No invasive fingerprint collection added. |
| Advertiser portal: targeting, ₹5000 budgets, CPM/CPC, real-time campaign metrics | NOT IMPLEMENTED | Deferred by owner until core/provider gates close; Coming Soon is not functionality. |
| Display advertising / sponsored prediction packages | NOT IMPLEMENTED | No sponsored monetization launch. |
| Premium subscription/ad-free/monthly bonus plan | NOT IMPLEMENTED | Deferred; no invented bonus amounts or live payment reuse. |
| Anonymized paid insights reports | NOT IMPLEMENTED | Deferred; data-consent/legal review required. |
| Optional rewarded ads: 5 coins, opt-in, max 3/day | NOT IMPLEMENTED | Deferred; no fabricated watch success or rewards. |
| Mobile-first web | PARTIAL | Authenticated layouts tested at 390/1440; wallet/leaderboard/proposal/market also 320. Native device, accessibility and full browser/device matrix still needed. |
| Installable PWA/native Phase-2 app (PDF p.10) | NOT IMPLEMENTED | No functioning manifest/service worker/native app found. |
| Legal launch review, DPDP/IT Act policy, separate advertiser terms | PARTIAL | Free-play/non-transferable copy corrected; counsel/consent/legal review not verified. No legal classification claim made. |
| 18+ participation enforcement | NOT IMPLEMENTED | Terms say 18+; server/registration enforcement and consent evidence are missing. Release blocker. |
| Official-result/dispute clauses and fraud-revocation operations | PARTIAL | Official-source/final-decision rules and moderation exist; dispute procedures and audited revocation policy need owner/legal validation. |
| Acquisition channels, beta/10k/50k/200k milestones and paying advertisers | NOT IMPLEMENTED | Business/go-to-market outcomes are not verified by source code; no outreach or fabricated adoption metrics. |
| Documentation aligned to approved rules | PASS | User guide, rules/terms, API, admin and deployment/migration guide updated; remaining gaps explicitly disclosed. |
| Full requested verification and every browser state combination | PARTIAL | Eight required commands pass; race unavailable. Main normal/error/invalid/empty/responsive/session flows tested; limits below prevent exhaustive/provider certification. |

## Product rules changed

- Paid coin purchase/staking removed from the game. Historical payment and stake records preserved; new submissions amount=0, no payment or balance prerequisite.
- New coin expiry changed from one year to six calendar months; expired funds cannot be restored by rejection/refund.
- Profile award changed from 250 to 30, once for actual fields. Historical paid rewards remain accounting history.
- Login-based streak rewards replaced with a separate prediction-day streak; daily login remains +10. Unspecified first/10/100-prediction achievement coin bonuses are now badges only.
- Signup/KYC/deposit referral milestones disabled; first prediction only, exactly 50 after 48h, at most 20. Signup 100 retained because it is explicit in the PDF.
- Generic answer/payout behavior replaced by 18 category/difficulty formats and exact payouts, trusted measured settlement, fixed snapshots and challenge doubling.
- Identity approval no longer automatically upgrades Gold or authorizes redemption without current phone/email verification.

## Database changes

Added PredictionStreak, CoinConsumption allocation/refund tracking, EconomyMigration audit records and profile full name. Extended Market with type/rule/range/evidence/news/challenge metadata; PredictionSubmission permits zero historical amount; challenge has a nullable unique market link; KYC stores identity/contact dates; withdrawals store encrypted code/provenance and fulfilment/delivery/dispatch/failure dates. Existing ledger/payment/history rows are retained. UTC is used for new writes and scheduler cutoffs. Schema and old-expiry transition details are in [Deployment_Guide.md](Deployment_Guide.md).

## API, frontend, news and prediction changes

[API_Documentation.md](API_Documentation.md) specifies free submissions, exact typed choices, profile/stats/batch/voucher/challenge endpoints, retired coin payments and protected settlement/fulfilment actions. [Admin_Guide.md](Admin_Guide.md) describes editorial review and real fulfilment operations.

Frontend removes paid checkout and stale economic claims, adds typed controls, proposal rules, challenge history, actual batches/voucher states, real profile completion, pending referrals and authenticated live events. Accessible application dialogs replace unsupported native prompt/confirm flows. Portfolio summary uses a registered identity-scoped endpoint, complete-history summary and paging; API errors cannot masquerade as empty history. Wallet/leaderboard mobile overflow fixed. Profile/leaderboard initials render locally without transmitting usernames to an avatar service. News preserves genuine source URLs/timestamps and stays unpublished until editorial review.

## Resolution changes

Settlement requires a locked market, canonical measured outcome, approved evidence on the configured source hostname and an observation timestamp after cutoff. Winner calculation handles declared formats, tolerances and closest-answer ties. Transactional settlement preserves the promised potential, updates participant/results history and posts ledger/notifications once; sources are attested by the authorized reviewer, not by AI.

## Reward economy changes

Every new earned credit gets a six-month UTC batch. Profile, daily login, prediction-day milestones and referral payouts use persistent transactional claims. Redemption refunds restore only still-valid original allocations. Daily login and prediction streak tables remain separate; historical awards and paid payment records are preserved.

## Redemption changes

The five exact tiers share the current annual identity/contact gate and FIFO allocation accounting. Tier fulfilment stores sourced voucher codes encrypted; protected state transitions and a persistent SMTP dispatch claim prevent fake Delivered states and duplicate sends. Contact verification, official supplier certification, positive staging email delivery and uncertain-delivery reconciliation remain open.

## Weekly challenge changes

Challenges link to reviewed markets before any activity. Participation commits with the standard prediction and snapshots a doubled reward. Cutoff closes the challenge; ordinary measured settlement completes results, pays winners and updates the real leaderboard/history. Admin creation and user results are available on the challenges page.

## Browser tests completed

Only `http://127.0.0.1:5174` with isolated SQLite data and development-seeded accounts. The frontend proxy and backend are local audit helpers, not shipped production business substitutes. No provider success was mocked. Disposable cutoffs/earned timestamps were deliberately aged for settlement and expiry tests; normal protected settlement and normal economic services performed the resulting actions.

| Flow | Observed result |
| --- | --- |
| Signup/login/logout | Mismatched passwords rejected; disposable referral signup succeeds, 100 welcome + 10 daily; wrong password 401; seeded user/admin login and logout work. |
| Dashboard/Markets | Actual market list, sorts and prediction streak; news unavailable explicitly; real second-user count updates. Legacy unreviewed details explain participation block. |
| Prediction details | Range invalid/valid, score pair, margin, two vs three nominees, closest price, four-option, binary first prediction. Six main category predictions settled correctly; no stake debit. |
| Portfolio/Predictions | Active and settled results; repaired stats endpoint shows 6 predictions, 100% settled win rate, 0 pending potential after settlement; pagination controls. |
| Wallet/expiry | Profile +30 once, daily +10 once, win credits, six-month dates, one reminder and one -100 expiry debit after two processing runs; 890 final valid coins after six wins/expiry. |
| Referral/streak | New referred user's first binary prediction; owner shows pending 50 with exactly two-day due time and zero paid; day-1 prediction streak separate from daily login. Multi-day/due-time/cap tests are backend regressions. |
| Weekly challenge | Completed; two actual participants receive 100 each, twice normal 50; results/history and connection status visible. |
| Redemption/KYC | Tier/confirmation form works; unverified redemption returns 403 with no debit; missing HyperVerge configuration returns explicit error, button recovers. No verification bypass or real voucher/email. |
| Rewards | Empty catalogue and empty historical redemptions render clearly, with link to five-tier wallet. No dummy inventory added. |
| Profile/Settings | Actual required field save awards 30 once; repeated save unchanged; stats/achievements/referral pending are real. |
| Leaderboard | Points/streak/winrate filters and empty search; local initials; mobile overflow retested. |
| Notifications | Actual win and expiry messages; mark-read/unread empty state works. |
| Support | FAQ corrected to free entry/real settlement; form inspected, no support email sent. Priority service not implemented. |
| User proposals | Range proposal submitted successfully and published after protected review. |
| Admin | Market settlement form paid actual winner; KYC/withdrawal/report empty states, analytics, hierarchy-disabled moderation, user search empty state and injected failure. |
| Authorization/session | Anonymous wallet redirects; normal user cannot stay on admin page. Ban/unban bug fixed: previous token yields 401/login redirect, fresh login works. Automated route tests cover unauthorized APIs. |
| Failure/loading | Injected 503 for dashboard, details/comments, portfolio/stats, rewards/history, wallet/batches/vouchers/KYC, notifications, challenges, profile/referrals and admin lists. Explicit error rendering. Delayed portfolio shows loading with disabled navigation and recovers. |
| Responsive/navigation | 390/1440 geometry checks across dashboard, market, portfolio, wallet, rewards, profile, leaderboard, notifications, settings, support, proposals, challenges; 320 checks on wallet/leaderboard/proposals/details. Mobile menu opens/closes. |

Browser coverage is not exhaustive for every loading/empty/permission combination on every page. Admin mobile interaction, successful provider KYC/contact/email delivery, live-news success, real voucher sourcing, seven real consecutive browser days and native-device/PWA behavior remain unverified. The audit is complete for the local flows above, not for those external/full-matrix requirements.

## Reproducible issues found and fixed

1. SQLite scheduler compared local-offset text with UTC cutoffs and locked a future market early. Root cause: inconsistent timestamp writes/query cutoffs. New database timestamps and controller/service cutoffs use UTC; offset cutoff regression and browser score flow pass. Historical timestamps require explicit migration review.
2. Portfolio/profile requested missing `/me/stats` (404); portfolio left win rate blank and interpreted API failure as empty data. Added caller-scoped full-history summary, ownership regression, paging and explicit failure state. Browser retest shows correct results.
3. Voucher/native admin prompts failed in the embedded browser. Exact console error below. Accessible form dialogs replace those prompts/confirmations; voucher gate and actual admin settlement retested.
4. Wallet/leaderboard overflowed mobile content by 42/35 pixels at 390. Wrapped headers/controls and constrained grid children; 390 and 320 retests show zero overflow.
5. Ban then unban revived old tokens. Ban now increments token_version; regression proves old token fails and fresh token works. Browser revocation retest redirects to login.
6. Stale stake/refund, binary-only proposal and bonus copy contradicted the PDF. Updated actual controls, publication requirements and rule/support text. Historical records were not erased.
7. Leaderboard avatar requests disclosed usernames externally. Local initials replace remote requests; leaderboard filter/search retested.

## Console and network findings

Exact reproducible runtime error before repair:

```text
Error: prompt() is not supported.
    at openWithdraw (http://127.0.0.1:5174/js/pages/wallet.js:125:18)
    at HTMLButtonElement.onclick (http://127.0.0.1:5174/wallet.html:69:138)
```

That historical entry remains in the tab log; no new error/warning was observed after the dialog fix. No unresolved runtime exception is claimed from the tested post-fix flows. Setup-only connection refusal occurred before the audit server started.

Exact HTTP observations: `/api/me/stats` 404 before the route fix, then 200; invalid prediction range/top-three 400; `/api/payments/redeem` 403 for incomplete current verification; `/api/kyc/start` 500 `HyperVerge credentials not configured`; `/api/news` 503 `News is not configured`; wrong login 401; revoked-session `/api/me`, `/api/wallet/history`, `/api/me/daily-login` 401. Deliberately injected failures were 503 with `Disposable audit: injected API failure` on the paths listed above. Successful proposal 201, challenge creation 201, settlement 200. These expected failures were visible rather than converted into fake success.

Sanitized proxy evidence: `audit-artifacts/pdf-sprint/network.jsonl` (method/path/status, no authorization headers, bodies or WebSocket token query). It includes local API driver requests as well as browser requests; it is not a browser HAR. Screenshot: `audit-artifacts/pdf-sprint/challenge-results.jpg`. Initial browser/provider observations made before proxy logging are recorded here separately.

## Tests added and results

`backend/tests/pdf_rules_test.go` covers calendar expiry/migration/refunds, free zero-balance submissions/nonzero rejection, concurrent submission, separate daily/streak rewards, concurrent one-time profile claims, referral milestones/delay/cap/concurrent processing, all 18 category payouts/typed outcomes, challenge settlement concurrency, all five voucher tiers/annual gate/encryption/failed SMTP dispatch duplicate protection, news freshness/dedup/editorial gating, caller-owned complete-history stats and ban-session revocation. `backend/services/cron_test.go` covers future UTC cutoff under a non-UTC local timezone. Existing obsolete economy/auth/cutoff tests were reconciled while preserving prior auth regression coverage.

Commands were run from `backend` for Go and repository root for frontend/Git, using a writable local Go cache:

| Exact command | Result |
| --- | --- |
| `go fmt ./...` | PASS, exit 0, no final formatting output. |
| `go vet ./...` | PASS, exit 0. |
| `go build ./...` | PASS, exit 0. |
| `go test ./...` | PASS, exit 0, services/tests pass; final run cached following a successful uncached run (tests 6.283s). PostgreSQL tests skip. |
| `go test -race ./...` | BLOCKED/FAIL, exit 1: `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`. Retried with CGO_ENABLED=1: `cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%`. No race pass. |
| `go mod verify` | PASS, exit 0: `all modules verified`. |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | PASS, exit 0 after approved public-network access; scanner v1.8.0, Go 1.26.8, zero reachable or imported-package vulnerabilities. One required-module advisory below. |
| `node scripts/check-frontend.mjs` | PASS, exit 0: 27 HTML pages, 24 script syntax checks, links/modules, structure, IDs, escaping/URL regressions. |
| `git diff --check` | PASS, exit 0; line-ending conversion warnings only. Rerun after final documentation/cleanup. |

Verbose vulnerability scan: GO-2026-5932 concerns unmaintained `golang.org/x/crypto/openpgp`, found in x/crypto v0.56.0, no fixed version. The application does not import/call that package. This is a dependency advisory qualification, not a claim of zero advisories or an exploitability guarantee.

Additional `go test ./tests -run TestPostgres -v -count=1` returned exit 0 with **all four PostgreSQL tests SKIP**, because `127.0.0.1:15432` refused connections. That is not a PostgreSQL pass. Read-only Docker info found no local docker_engine pipe; no containers were started.

## Remaining PDF gaps and production gates

Next work, in dependency order: implement/certify phone OTP and email confirmation without an approval shortcut; certify official voucher sourcing, TLS SMTP/inbox delivery and failed/uncertain dispatch operations; implement 18+ enforcement and consent evidence with legal review; rehearse/review historical transitions on PostgreSQL; run race/locking/container checks; certify real result/news providers and fallback behavior. Priority support, Diamond ad placement, privacy-appropriate automated abuse controls and PWA remain product gaps. Begin the explicitly deferred advertiser/subscription/insights/rewarded-ad work only after the core gates are accepted.

| Separate release gate | Final status |
| --- | --- |
| Credential rotation/revocation | NOT VERIFIED; removing files/retiring endpoints is not revocation. |
| PostgreSQL/container testing | BLOCKED/NOT VERIFIED; tests skip, Docker engine unavailable. |
| External-provider staging (KYC/contact, SMTP, news/results, voucher supplier, Google; any separately approved payments) | NOT VERIFIED; no live provider credentials used. |
| Production-data reconciliation | NOT VERIFIED; no production balances/records changed. |
| Race detector | BLOCKED; CGO compiler unavailable. |
| Production deployment | NOT PERFORMED. |
| Commit/push | NOT PERFORMED, as requested. |

Local fixes and successful tests can be reviewed now. Release approval must wait for the remaining gates and PDF gaps; neither the local audit nor this report authorizes deployment.
