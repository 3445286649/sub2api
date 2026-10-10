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

Within one SubAPI installation, the legacy allocator and gateway cannot accept
BSC orders simultaneously. A TRON-only gateway can coexist with legacy BSC.
Before switching from legacy
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
do not substitute a configuration bypass. For an already paid legacy order whose
original intent was lost, an audited chain review may record a separate
`usdt_bsc.recovered_reservation_evidence` object with schema version 1 and source
`chain_receipt_review`. It must contain BSC contract, address, exact atomic amount,
transaction hash, event index, block number/hash/time, review time and provenance
digest. Export accepts it only when the order's receipt binds the same chain,
hash, order, contract, amount and event; the address matches the retained QR field;
and chain time is within the original order window and no later than recorded
payment. Settlement after expiry remains valid if the chain transfer was in time.
An existing original intent takes precedence and conflicting recovery evidence is
rejected. The evidence digest describes provenance, not authentication. Recovery
does not restore an original quote, rate or lock time, nor alter paid state or
credit the order again. Unpaid orders cannot use this recovery path. Other missing
history still requires a new observation address pool.

When another site continues to allocate six-decimal legacy BSC quotes to the same
wallet, a one-time import alone cannot prevent future allocation overlap. New BSC
gateway quotes therefore use quote version 2, rounding `ceil_6dp_bsc_7`: the
existing six-decimal ceiling plus a tail of 1-899 micro-USDT, followed by exactly
one fixed seventh digit `1`. In atomic BSC units, the remainder modulo `10^12`
must be `10^11`. This namespace never equals a legacy six-decimal quote. Wallets
must preserve all seven displayed digits; rounding to six digits cannot pay the
new order. TRON retains quote version 1 and `ceil_6dp`. Existing version 1 orders
remain readable. Version 2 paid proofs reject collection-wallet self-transfers,
which do not establish incoming funds. Each site's balances and transaction
receipts remain separate; namespace isolation is not cross-site data replication.

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
