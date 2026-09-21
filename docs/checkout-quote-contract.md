# Confirmed cart quotes

All order creation now requires an authenticated, explicitly confirmed quote.
Deploy the API and Android client together. The old empty-body checkout request
is rejected with `quote_confirmation_required`; there is no legacy bypass.

## Create and display a quote

`POST /api/v1/cart/quote`, with the user's Bearer token and `{}`, returns:

```json
{
  "quote_id": "quote_example",
  "cart_version": 7,
  "currency": "CNY",
  "amount_scope": "merchandise",
  "fulfillment_status": "unconfirmed",
  "pricing_version": "cny-basket-v1",
  "expires_at": "2026-09-06T12:05:00+08:00",
  "total_amount": "32.57",
  "discount_amount": "5.01",
  "pay_amount": "27.56",
  "lines": [],
  "cart": {"items": [], "summary": {}},
  "confirmation_token": "opaque-value-returned-only-to-the-user-interface"
}
```

The example omits the actual line items for brevity. The real response includes
the selected products, exact SKU labels/specifications, quantities, current
prices, discounts and summary. The UI must render this cart, not a cached cart
from a previous page. The token is random, its hash is stored, and the response
must not be cached. `GET /cart/discount-preview` returns the same quote contract
for existing UI callers. A quote lasts at most five minutes and ends sooner if
an included active offer expires. Creating a quote reserves neither stock nor coupons.

## Confirm once, retry the same confirmation

After displaying the quote and receiving the user's button press:

```http
POST /api/v1/orders:checkout
Authorization: Bearer <user-token>
Content-Type: application/json

{"quote_id":"quote_example","confirmation_token":"<token from that quote>"}
```

The server derives ownership from the authenticated account. Under database
locks it checks the token, expiry, monotonic cart version, live SKU availability,
SKU specifications, prices and applicable promotion/coupon policy fingerprint.
It then reserves stock/coupons, creates the merchant orders and payment records,
removes the selected cart lines and stores the receipt in one transaction.
All money calculation and allocation use integer CNY cents.

Repeated valid submissions of the same quote return the original creation
receipt, including after its original expiry time. They do not create another
order or reapply a coupon. Fetch `/orders/{order_id}` for its current status.
Network/5xx errors must retain the quote and token; retrying with a newly created
quote would lose this operation identity. The Android confirmation page retains
the same quote across retry and saved-instance restoration.

`quote_changed`, `quote_expired`, `cart_unavailable` and
`quote_confirmation_required` are definitive rejections. Obtain a new quote,
display it and require another button press. Do not automatically confirm a
replacement quote. If quote retrieval fails, do not substitute an undiscounted
price or proceed using a stale local cart.

The Agent's `checkout` tool prepares a quote and directs the user to confirmation;
it does not submit an order. Its observation explicitly contains
`requires_confirmation=true` and `order_created=false`. The confirmation token
is removed before the quote enters model context or Agent traces.

## Pricing policy version cny-basket-v1

- Product/category/merchant scope filters the eligible original line amounts.
  Merchant ownership also narrows product/category offers. Unknown scope/type or
  malformed amounts fail quotation rather than silently applying a broad discount.
- Thresholds use the original eligible subtotal. Fixed discounts and percentage
  discounts apply to the remaining eligible subtotal. `discount_rate=0.1000`
  means a 10% reduction, preserving the existing API's fraction-off semantics.
- Stackable offers apply in stable `(kind, id)` order: coupons then promotions.
  A non-stackable offer is exclusive across the whole checkout. The engine chooses
  the stackable bundle or a single exclusive offer, whichever gives the larger
  discount; it does not sum incompatible offers.
- Existing coupons default to `stackable=false`. Set the stored coupon policy
  deliberately to enable stacking. The wallet API includes this policy.
- Discounts are allocated to eligible line items in cents, then summed into each
  merchant order. Allocation preserves the full discount without rounding loss
  and never makes a line or order negative.
- One coupon may span several merchant orders in a confirmed checkout. Its
  allocation records start reserved. Paying any allocation consumes the coupon.
  Canceling or expiring all unpaid allocations returns it to unused. Partial
  cancellation leaves remaining order prices unchanged; once any allocation is
  paid the coupon cannot be returned to the wallet. Expired returned coupons
  remain ineligible for a subsequent quote.

This contract covers the repository's product-price and promotion amounts.
Shipping fees and destination-specific fulfillment are not yet modeled; those
remain in the full implementation ledger and must be resolved before claiming
that a quote includes real-world delivery costs. Production simulated payment
remains disabled until a real payment boundary is implemented.

## Migration and verification

Apply `backend/migrations/003_confirmed_cart_quotes.sql` once after migration 002,
or use the idempotent Go migration path. Drain old API instances first: they do
not bump cart revisions or implement the new confirmation contract.

Backend checks with a disposable MySQL server:

```sh
cd backend
XZXG_TEST_MYSQL_DSN='root@tcp(127.0.0.1:13306)/?parseTime=true' go test -race ./src/store ./src/agent ./src/httpapi ./src/pricing
```

Tests cover exact cent allocation, scope/exclusivity, multiple merchant orders,
concurrent duplicate confirmations, ownership/token checks, cart changes that
are later reversed, price/SKU/policy changes, quote expiry, coupon allocation
failure rollback, cross-merchant payment/cancel races, and timeout coupon release.
They also verify that the Agent cannot expose or self-submit a confirmation token.

## Recovery and shared product inventory

Storage failures are returned as retryable service failures, not as a definitive
`cart_unavailable` rejection. The same quote and confirmation credential remain
usable after the failed transaction rolls back. Do not generate a replacement
quote automatically after a timeout or temporary service failure.

Availability includes the combined quantity of all selected SKUs belonging to
the same product. Two individually available variants cannot bypass the parent
product's total stock limit. Final reservation retains guarded SQL deductions.

The integration suite injects failures at SKU/product reservation, order,
payment, order-item and inventory-reservation insertion, and cart removal. Each
case checks rollback, coupon/quote state, unchanged stock/cart, then confirms
and replays the original quote without creating another order.
