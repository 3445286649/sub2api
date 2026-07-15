# UpstreamOps Scheduling Hold API v1

Status: frozen for Sub2API and UpstreamOps contract tests
Contract version: `2026-07-15`
Authentication: existing Sub2API Admin authentication
Owner: the server fixes the owner to `upstreamops`; clients cannot submit an owner

This API is an optional enhancement. Clients must probe capabilities and fall back to the official Admin API behavior when `external_holds` is false or the capability endpoint is unavailable.

## Capability probe

```http
GET /api/v1/admin/scheduling/capabilities
```

Successful `data` payload:

```json
{
  "contract_version": "2026-07-15",
  "external_holds": true,
  "external_hold_owner": "upstreamops",
  "default_lease_seconds": 900,
  "minimum_lease_seconds": 60,
  "maximum_lease_seconds": 3600,
  "maximum_cumulative_lease_seconds": 14400,
  "capacity_guard": true,
  "optimistic_concurrency": true,
  "idempotency": true,
  "lease_expiry": true,
  "probe_while_held": true,
  "scheduler_outbox": true
}
```

## Read scheduling state

```http
GET /api/v1/admin/accounts/:id/scheduling-state
```

The response returns manual account state, internal temporary protection, the UpstreamOps hold, health/probe evidence and the final effective state. It never returns credentials, API keys, tokens or complete upstream error bodies.

## Create or renew a hold

```http
PUT /api/v1/admin/accounts/:id/scheduling-holds/upstreamops
Content-Type: application/json
```

```json
{
  "decision_id": "ops-749-20260715-001",
  "reason_code": "sustained_ttft",
  "lease_until": "2026-07-15T16:30:00Z",
  "expected_account_updated_at": "2026-07-15T16:10:00Z"
}
```

Rules:

- `decision_id` is 1-64 characters and only contains letters, digits, `.`, `_`, `:`, or `-`.
- Supported reasons are `auth_invalid`, `quota_exhausted`, `upstream_unreachable`, `sustained_5xx`, `sustained_ttft`, and `manual_approved`.
- Lease duration is 60-3600 seconds. Continuous renewals cannot exceed 14400 seconds from the first hold.
- The same decision and same payload are idempotent. Reusing a decision with a different payload returns `409 HOLD_DECISION_CONFLICT`.
- Account `updated_at` drift returns `409 ACCOUNT_STATE_DRIFT` before applying a new hold.
- Manual `schedulable=false` returns `409 MANUAL_SCHEDULING_DISABLED`.
- The hold must leave at least one effective account in every affected group or ungrouped platform pool. Otherwise it returns `409 CAPACITY_GUARD_BLOCKED`.
- A successful hold schedules a health probe, updates the scheduler account projection and emits a scheduler outbox event.

## Release a hold

```http
DELETE /api/v1/admin/accounts/:id/scheduling-holds/upstreamops
Content-Type: application/json
```

```json
{
  "decision_id": "ops-release-749-20260715-001",
  "expected_hold_decision_id": "ops-749-20260715-001"
}
```

Rules:

- A repeated release decision is idempotent.
- A different active hold returns `409 HOLD_RELEASE_CONFLICT`.
- Releasing a missing, released or expired hold succeeds without changing manual or internal state.
- Releasing an UpstreamOps hold never sets `accounts.schedulable=true` and never clears `temp_unschedulable`.

## Scheduling state payload

```json
{
  "account_id": 749,
  "account_updated_at": "2026-07-15T16:10:00Z",
  "manual_schedulable": true,
  "internal_blocked": false,
  "internal_reason_codes": [],
  "external_hold": {
    "owner": "upstreamops",
    "decision_id": "ops-749-20260715-001",
    "reason_code": "sustained_ttft",
    "status": "active",
    "lease_until": "2026-07-15T16:30:00Z",
    "active": true
  },
  "effective_schedulable": false,
  "effective_reason_codes": ["external_hold"],
  "health": {
    "score": 72,
    "status": "degraded",
    "consecutive_successes": 0,
    "last_checked_at": "2026-07-15T16:09:00Z",
    "next_probe_at": "2026-07-15T16:11:00Z",
    "probe_enabled": true
  }
}
```

## Error contract

All errors use the existing Sub2API envelope: `code`, `message`, `reason`, and optional string `metadata`.

| HTTP | reason | Meaning |
| --- | --- | --- |
| 400 | `INVALID_ACCOUNT_ID` | Invalid path account ID |
| 400 | `INVALID_HOLD_REQUEST` | Malformed JSON or missing required field |
| 404 | `ACCOUNT_NOT_FOUND` | Account does not exist |
| 409 | `ACCOUNT_STATE_DRIFT` | `updated_at` differs from the expected value |
| 409 | `HOLD_DECISION_CONFLICT` | A decision ID was reused with another payload |
| 409 | `HOLD_RELEASE_CONFLICT` | Release expected another active hold |
| 409 | `MANUAL_SCHEDULING_DISABLED` | Manual schedulable state already blocks the account |
| 409 | `CAPACITY_GUARD_BLOCKED` | Hold would violate the minimum remaining capacity |
| 422 | `LEASE_OUT_OF_RANGE` | Lease or cumulative hold duration exceeds the contract |
| 422 | `INVALID_REASON_CODE` | Reason code is not in the allowlist |
| 503 | `SCHEDULING_HOLD_UNAVAILABLE` | Hold storage or propagation is unavailable |

## Expiry and probe semantics

- Expired leases are ineffective immediately by timestamp comparison.
- A background expiry pass persists `expired`, clears the account projection and emits an outbox event.
- Held accounts remain eligible for background probes even though they are excluded from user scheduling.
- Internal health recovery can only clear internal temporary protection. It cannot release an UpstreamOps hold.
- Sub2API never infers an UpstreamOps release decision. UpstreamOps releases its own hold after evaluating recovery evidence.
