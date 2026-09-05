# Shopping production migration and verification

This is an incremental implementation. The full remaining scope is tracked in
[the acceptance ledger](production-shopping-plan.md). Passing the checks below
does not establish that all production requirements have been completed.

## Inventory reservation rollout

1. Back up the database and drain checkout/payment/cancellation traffic. Do not
   run old and new transaction implementations concurrently during this migration.
2. Apply `backend/migrations/002_inventory_reservations.sql`, or run the current
   API migration path (`RUN_MIGRATIONS=true`) in a controlled maintenance window.
   Regular production startups default to not running migrations.
3. Deploy the new API. New order items receive reservation records in the checkout
   transaction. Existing pending orders are adopted, without another deduction,
   when they are paid or closed while holding their order row lock.
4. Query `GET /api/v1/admin/inventory/audit` with an administrator account. Results
   distinguish missing legacy reservations, order/reservation mismatches and
   negative stock. `truncated=true` means the response is not a complete audit.
5. Investigate historical canceled/expired orders against inventory movement or
   warehouse records. The previous implementation did not release their stock.
   The migration deliberately does not invent stock adjustments: inventory may
   have already been manually corrected. Record the evidence and adjustment in
   the business's inventory system before reconciling such history.

The API sweeps at startup and every 30 seconds, processing up to 128 expired
orders in each batch with a 20-second timeout. Order reads retain a bounded lazy
expiry pass. Failed transactions remain retryable and the periodic path logs
errors. Monitor expiry errors and overdue pending-order backlog; size the batch
and scheduling interval to the measured order volume.

Payment, cancellation and timeout share the order-row lock. Closing an unpaid
order atomically releases its SKU/product inventory, transitions reservation
records, closes the order and updates pending payments. A paid order cannot be
released by this path. Repeated cancellation returns the terminal order. Database
failures roll back the entire close transaction.

## Payment boundary

`APP_ENV=production` always disables simulated payment, regardless of
`ENABLE_DEMO_PAYMENTS`. Without a real provider integration the API returns
`payment_not_configured` and leaves the order unpaid. The Agent also reports this
boundary instead of claiming that payment succeeded. Development deployments may
enable simulation with `ENABLE_DEMO_PAYMENTS=true`; only `mock` and `mock_balance`
methods are accepted. Do not present this as a real payment-provider integration.

## Tests

Use a disposable MySQL 8 server. `XZXG_TEST_MYSQL_DSN` must identify a server whose
test account can create and drop databases. Tests create independent
`xzxg_test_*` schemas and drop only those schemas. Never pass production credentials.

```sh
cd backend
XZXG_TEST_MYSQL_DSN='root@tcp(127.0.0.1:13306)/?parseTime=true' go test ./...
```

Without that environment variable MySQL integration tests are explicitly skipped;
ordinary unit-test success is not inventory verification. Integration cases cover
rollback after an injected release failure, concurrent repeated cancellation,
payment/cancellation races, legacy expiry/payment/cancellation races, insufficient
stock across a cart, final-unit availability, audit detection, and disabled payment.

From the repository root:

```sh
node --test quality/evals/stream_state.test.mjs
```

The shared evaluation reducer consumes both `content_delta` and `block_delta`
cards. It rejects incomplete, malformed, mixed-run and error streams. Shopping
assertions can require exact displayed IDs and exact cart SKU/quantity/selection
sets, including empty sets and unchanged carts. Cart checks read the business API
independently of the Agent's reply. E2E reports include coverage and transport
failures; their default gate requires full coverage and full passing results.
Existing cases without business assertions are uncovered, not implicit successes.
