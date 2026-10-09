# BEpusdt merchant v2 integration

The `bepusdt` provider offers `usdt_gateway`, with network IDs `bsc_mainnet`
and `tron_mainnet`. It is separate from the existing `usdt_bsc` provider.
The gateway keeps only observation addresses. No wallet private key or automated
refund is supported by this adapter.

Provider configuration is encrypted using the existing payment encryption key:
`apiUrl`, `merchantId`, `keyId`, distinct `apiSecret` / `callbackSecret` (at least
32 bytes each), and `allowedNetworks` as comma-separated mainnet IDs. API URLs
require HTTPS; loopback HTTP is accepted for local fixtures. Redirects are rejected.
Administrator `POST /api/v1/admin/payment/providers/{id}/check` calls the signed,
read-only gateway health endpoint; it never creates a payment.

The user creates an order with `payment_type=usdt_gateway`, `crypto_network`,
and a canonical UUID `client_request_id`. Retries preserve that UUID. Changed
amount, plan, order type, or network with the same UUID return a conflict. Local
orders commit before remote creation; unknown results keep their original order
number and recover through authenticated query and idempotent create. The checkout
response and order GET expose `create_stage`, `gateway_status`, crypto amount,
network, address, and confirmation progress without gateway credentials.

New gateway subscription orders use CNY plans directly. USD plans (including the
existing unlabeled plans displayed as USD) require the explicit existing
`subscription_usd_to_cny_rate` setting. Other plan currencies are rejected.
Original plan price and fulfillment remain fixed; only the payment quote converts.

Disabling either crypto provider stops new orders. Historical orders continue to
query, accept verified payments, and retry fulfillment. Configuration and deletion
are frozen after historical references; credentials or address changes require a
new instance. Names, limits, and enablement remain editable. The create and admin
transactions lock the same provider row; no remote call runs inside order creation.

The gateway's PAID-only callback is
`POST /api/v1/payment/webhook/bepusdt`. The adapter verifies merchant identity,
timestamp, nonce and HMAC, then queries the original pinned provider instance.
It compares immutable intent, quote, asset, network, contract, atomic amount,
address, in-window chain block time, transaction identity and confirmations.
Minimum confirmations are BSC 20 / TRON 30. Receipt reservation, PAID and audit
share one transaction. Only a durable payment returns
`{"ack_event_id":"..."}`. Fulfillment failures retry independently through the
existing fulfillment lease and expiry runner. Unknown orders, invalid signatures,
query failures and rejected payment evidence never receive a success ACK.

`crypto_payment_receipts` reserves `(chain_id, tx_hash)` across both crypto
providers, including refunded and failed-fulfillment payments. Receipts do not
cascade with order deletion. Migration 243 adds the client request index, receipts
and persistent callback nonces. The expiry runner backfills old paid BSC receipts;
missing hashes or conflicting historical ownership fail closed and require review.

## Shared BSC address and channel switching

The legacy allocator and the gateway cannot accept BSC orders simultaneously.
A TRON-only gateway can coexist with legacy BSC. Before switching from legacy
BSC to gateway BSC, disable legacy new orders and drain unresolved local create
operations. Export all historical exposed tuples, permanently import them using
the gateway's offline tool, then verify the signed health import digest. The
adapter compares all distinct tuples using this canonical sorted line format:

`bsc_mainnet|<lowercase-USDT-contract>|<lowercase-address>|<atomic-integer>`

The digest is SHA256 of UTF-8 lines joined with LF, without a final LF. An
administrator downloads these rows with
`GET /api/v1/admin/payment/legacy-bsc-reservations`; the response includes
`count`, `digest`, and `reservations`.
Import source must be `subapi_usdt_bsc`. Missing old intent evidence is a blocker;
do not substitute a configuration bypass. Existing old instances with lost
history should use a new observation address pool.

When switching back to legacy BSC, disable gateway new orders and resolve every
unknown gateway quote first. The legacy allocator permanently excludes every
historical gateway amount on that address, regardless of status or age. Shared
transaction receipts also prevent one transfer from crediting both providers.
Never reset receipt or permanent allocation tables to restore channel capacity.

## Validation

`go test -tags=unit ./internal/payment/... ./internal/service ./internal/handler
./internal/handler/admin` runs adapter and payment regressions. Optional
`SUBAPI_BEPUSDT_TEST_DSN` must identify a dedicated fixture database. Run
`go test -tags=unit ./internal/service -run TestBepusdtPostgres` to check PostgreSQL
row locking, concurrent intent uniqueness, channel enablement serialization, and
receipt/audit rollback. Tests create and drop their own random schema.

Real mainnet receipts, RPC behavior, recovery, and notification delivery still
require candidate-environment verification. Configuring the adapter alone is not
evidence that a payment can be safely accepted on a live chain.
