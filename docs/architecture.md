# Target architecture (Phases 3–6; not implemented in Phase 1)

Each service owns a PostgreSQL schema. A single local PostgreSQL instance can host
all three with separate roles and grants. Kafka carries integration events, not
synchronous cross-service database lookups. There are no distributed transactions.

## Event flow

1. Order commits `PENDING` plus `order.created` in its outbox (Phase 4).
2. Inventory consumes creation and reserves all requested products in one database
   transaction. Lock product rows in stable product-ID order and use conditional
   stock updates so two requests cannot consume the same final unit. Roll back
   the whole reservation if any item is unavailable.
3. Inventory emits `inventory.reserved` or `inventory.rejected` through its outbox.
4. Payment consumes reservations, durably records one simulated result per order,
   and emits `payment.succeeded` or `payment.failed` through its outbox. Failure
   simulation is configurable and its first durable outcome survives retries.
5. Order consumes results. Inventory also consumes `payment.failed` and releases
   each reservation once. Release is eventually consistent with the failed order.

Planned JSON envelope:

```json
{
  "event_id": "opaque-unique-id",
  "version": 1,
  "type": "order.created",
  "occurred_at": "2026-10-24T10:00:00Z",
  "correlation_id": "request-or-workflow-id",
  "order_id": "opaque-order-id",
  "data": {"customer_id": "customer-1", "items": []}
}
```

The example illustrates the envelope only; real creation events must contain
validated items, total cents, and the currency contract. Partition records by
`order_id`. Each service has its own consumer group. Inventory uses the creation
and payment streams; Payment uses reservations; Order uses result streams. Topic
retention, schema evolution and unknown-version handling must be configured and
tested before reliability is claimed.

Planned transitions:

| Current | Event | Next |
| --- | --- | --- |
| PENDING | inventory.reserved | RESERVED |
| PENDING | inventory.rejected | FAILED |
| PENDING or RESERVED | payment.succeeded | CONFIRMED |
| PENDING or RESERVED | payment.failed | FAILED |
| terminal state | duplicate or stale earlier-stage event | unchanged |

Kafka does not order events across topics. A payment result may reach Order before
the reservation event; Order must accept that result without later regressing.
Conflicting terminal results are an invariant violation to investigate, not an
instruction to overwrite the terminal state. `CANCELLED` is reserved for a future
explicit cancellation workflow and compensation; no phase 1 endpoint sets it.

## Failure semantics to implement in Phase 4

| Failure | Intended recovery |
| --- | --- |
| Kafka unavailable | Database transaction commits with outbox row; publisher retries with bounded exponential backoff and jitter. Expose backlog; define admission limits before disks fill. |
| Crash before database commit | Transaction rolls back; uncommitted Kafka offset is redelivered. |
| Crash after commit, before offset commit | Redelivery hits an inbox event-ID constraint in the same transaction as domain changes; acknowledge without repeating work. |
| Publisher crashes after send, before marking sent | Outbox publishes again; consumers deduplicate. Delivery is at least once. |
| Duplicate message | Durable inbox handles event-ID duplicates; unique order reservation/payment constraints also protect against semantically duplicate events with different IDs. |
| Payment failure | Order becomes FAILED; Inventory releases reservation in an idempotent transaction and emits any required audit event. |
| Stock exhausted | Reject entire order without partial reservation; Order becomes FAILED. |
| Competing for last item | Row locks/conditional writes allow one successful reservation; loser receives inventory.rejected. |
| Poison event / retries exhausted | Publish original event and failure metadata to DLQ before committing source offset. If DLQ write fails, retain source offset for retry. Replay is an explicit operation. |

Outbox selection must coordinate multiple publisher replicas. Consumer offsets are
committed only after the domain transaction or durable DLQ handoff. Transient
errors receive finite local retries and backoff; prolonged outages pause/replay
without silently dropping records. Domain rejections are results, not retryable
infrastructure errors. There is no exactly-once claim.

Read APIs return current database state, which can lag downstream processing.
Clients poll order status. During outages, `PENDING` or `RESERVED` can persist;
timeouts do not imply payment failure. Reconciliation and reservation expiry need
an explicit policy before introducing expiration, to avoid releasing paid stock.

Request correlation IDs will pass into event envelopes and logs. Redis is useful
for short-lived rate-limit counters across Payment replicas, but never the source
of truth for payment outcomes or durable deduplication. An unavailable limiter
will defer payment processing rather than invent a payment rejection.
