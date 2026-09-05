# Production shopping implementation and acceptance ledger

Base: `36494dc5d48dac9b1e360cba52ba918646192d56`. Scope: implement the complete shopping-agent review, not only the first passing fixes. This file records unfinished work explicitly; checkboxes require implementation and relevant verification.

## Requirements

- [ ] R01 Task-aware planning: compose read capabilities for comparisons, reviews and offers; budget by task; keep write authorization separate.
- [ ] R02 Persistent shopping task state: versioned hard constraints, soft preferences, exclusions, recipient, task switching and user corrections; display-set/SKU references; safe memory fallback.
- [ ] R03 Typed catalog constraints: exact money, per-item/whole-task budgets, attributes and availability; details/SKU tools; unknown is not satisfied.
- [ ] R04 Shared image/text candidate eligibility; preserve similarity evidence and distinguish similar from identical products.
- [ ] R05 Knowledge scope, provenance, effective dates and full policy exceptions; enforce retrieval filters and source lifecycle.
- [ ] R06 Evidence-backed recommendation outputs and commercial-role boundaries; conservative fallback with hard constraints preserved.
- [ ] R07 UI/task event synchronization and result-aware followups; canonical stream reducer used by evaluation.
- [ ] R08 Precise action target, SKU, quantity and authorization; idempotent operation receipts and confirmation bound to immutable parameters.
- [ ] R09 One quote calculation for preview and checkout: eligibility, stacking, exact totals, version/expiry, coupon reservation/redemption/release.
- [x] R10 Inventory reservation lifecycle: payment, cancellation and expiry; concurrency, exactly-once release and reconciliation.
- [ ] R11 Durable run/operation results, reconnect/replay, stable retry IDs, cancellation propagation and consistent terminal states.
- [ ] R12 Task latency/token/cost budgets, workload admission, shared quotas where configured, useful metrics and graceful degradation.
- [ ] R13 Context-aware risk screening, untrusted-content boundaries, trace minimization/retention and user data lifecycle.
- [ ] R14 Business evaluation: exact displayed sets and actual state mutations; negative assertions, fault/retry/multiturn tests, coverage and business outcomes.
- [ ] R15 Version-bound release evidence, explicit candidate promotion, rollback and controlled iteration; delayed outcome feedback without automatic reward hacking.
- [ ] R16 Payment boundary: safe provider handoff or disabled production payment, verified result contract; demo mode explicitly isolated. Anonymous consultation/identity merge boundaries documented and implemented as applicable.
- [ ] R17 Build/test backend, web and Android changes; integration checks for transactional and recovery invariants; synchronize operating and architecture docs.
- [ ] R18 Commit and push all verified work to GitHub; inspect remote state; no claim of full completion while any requirement lacks evidence.

## Acceptance scenarios

1. Change budget without losing other constraints; negative feedback changes candidates; recipient/task switching does not contaminate preferences.
2. Add exactly the selected black 256G SKU twice, with no extra products; unresolved references cannot become write targets.
3. Image plus text respects budget/exclusions; absent candidates and retrieval outages preserve hard constraints.
4. Quote, coupon and order amounts agree; price/cart changes invalidate previous confirmation.
5. Cancellation/expiry releases only unpaid reserved inventory once; racing payment and closure preserve stock.
6. Disconnect after a successful operation, retry, and retrieve the same receipt without another mutation.
7. Content-only product cards cannot escape evaluation; no extra SKU/quantity/action passes positive-only assertions.
8. Untested or stale candidate versions cannot be published; rollback restores a complete pinned configuration.

## Verification log

- Repository inspected at the base above; clean checkout on `codex/production-shopping`.
- Baseline `go test ./...` passed before edits.
- 2026-09-06: R10 implemented with per-item reservation records, atomic close,
  guarded payment/expiry transitions, periodic expiry and an admin reconciliation audit.
  Real MySQL 8.4.11 integration tests passed for eight concurrent cancellation attempts,
  20 payment/cancel races, legacy expiry/payment/cancel race, injected SQL failure rollback,
  insufficient stock rollback, last-unit stock status and audit corruption detection.
- Full backend `go test ./...` passed with `XZXG_TEST_MYSQL_DSN` configured.
- R07/R14 partial: canonical SSE evaluation reducer and exact cart/display assertions;
  four Node test groups passed. Full business datasets and client recovery are still pending.
- R16 partial: production simulation blocked in Store, HTTP and Agent; real MySQL test
  proves the order stays unpaid and stock reserved. Provider handoff and anonymous boundaries
  remain to be addressed before closing R16.
- Migration procedure and historical-stock reconciliation boundary recorded in
  `docs/production-shopping-operations.md`.

## Corrections to initial analysis

- `executeTool` already checks cancellation. Cancellation work must address propagation and race/result semantics, not claim no tool guard exists.
- Checkout already uses a transaction, row locks and guarded stock deductions. The missing reservation release lifecycle is the defect.
- Image vectors already have a minimum-score threshold. Missing shared business constraints are the defect.
- Payment is currently simulated; production claims require a provider handoff, not a fabricated successful payment.
