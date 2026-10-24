# OrderFlow

A Go learning project for event-driven order processing and Kubernetes operations.
**Current milestone: Phase 1 (Order API and PostgreSQL).** Kafka, Inventory,
Payment, Docker images, Compose, Kubernetes, and metrics belong to later phases.

## Architecture

The target system has three independently buildable services in a Go monorepo:

```text
Client -> Order REST API -> orders schema + outbox -> Kafka: order.created
                                                      |
                                                 Inventory
                                           inventory schema + outbox
                                                      |
                               inventory.rejected / inventory.reserved
                                                               |
                                                            Payment
                                                     payments schema + outbox
                                                               |
                                           payment.failed / payment.succeeded
```

Order consumes inventory and payment results. Inventory consumes payment failure
to release reservations. Each service will own its schema and database role;
services will communicate through events rather than reading each other's tables.
Redis will support a temporary payment rate limiter; PostgreSQL will retain durable
payment idempotency records so Redis expiry cannot cause a duplicate charge.

See [architecture and failure semantics](docs/architecture.md) for the target
event contract, lifecycle, and reliability decisions, and [milestones](docs/milestones.md)
for the implementation sequence. These describe future work explicitly.

## Repository

```text
cmd/order-service/       executable and explicit migration command
internal/order/         domain rules, HTTP API, PostgreSQL adapter, unit tests
migrations/             embedded SQL bootstrap migration
tests/                  opt-in PostgreSQL integration test
docs/                   architecture, milestones, learning and verification
Makefile                optional Linux/WSL shortcuts
.env.example            local environment examples (not loaded automatically)
```

Later phases add `cmd/inventory-service`, `cmd/payment-service`, `pkg/events`,
`deploy/docker`, `deploy/k8s`, and load-test scripts when they have working content.

## Run Phase 1 (Linux / WSL)

Prerequisites: Go 1.26+, PostgreSQL 17, and `curl`. Development was built with
the installed Go 1.26.5 toolchain. Docker Desktop can supply PostgreSQL while the
Go process runs on the host; Dockerizing the application is Phase 2.

From the repository root:

```bash
# Local learning credentials only. Start Docker Desktop first.
docker run --name orderflow-postgres \
  -e POSTGRES_USER=orderflow -e POSTGRES_PASSWORD=orderflow_local \
  -e POSTGRES_DB=orderflow -p 127.0.0.1:5432:5432 \
  -v orderflow-pgdata:/var/lib/postgresql/data -d postgres:17
docker exec orderflow-postgres pg_isready -U orderflow -d orderflow
# Wait for pg_isready to report accepting connections before continuing.
export DATABASE_URL='postgres://orderflow:orderflow_local@localhost:5432/orderflow?sslmode=disable'
export HTTP_ADDR=':8080'
go mod download
go run ./cmd/order-service migrate
go run ./cmd/order-service
```

For an existing PostgreSQL server, create the `orderflow` role/database and set
`DATABASE_URL` accordingly. TLS is disabled only for the local example. The Phase 1
role owns the schema; separate migration/runtime privileges will be introduced with
the other service roles. `.env.example` documents configuration; the executable
reads environment variables, not `.env` files.

PowerShell uses `$env:DATABASE_URL='postgres://...'` and `$env:HTTP_ADDR=':8080'`.
The Go commands work unchanged. For Windows HTTP commands use `curl.exe` or run
the examples below in WSL.

In a second terminal:

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -i -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' -H 'X-Correlation-ID: demo-1' \
  -d '{"customer_id":"customer-1","items":[{"product_id":"book","quantity":2,"unit_price_cents":1500}]}'
# Substitute the ID from the response:
curl http://localhost:8080/orders/ORDER_ID
curl http://localhost:8080/orders/ORDER_ID/status
```

Creation returns `201`, `Location: /orders/{id}`, and a `PENDING` order with
`total_cents: 3000`. Fetching an unknown ID returns `404`. Invalid input returns
`400`; bodies above 64 KiB return `413`. Unknown JSON fields and multiple JSON
values are rejected. IDs are random opaque strings. All timestamps are UTC.
`/healthz` checks the HTTP process; `/readyz` checks database/schema availability.
Ctrl+C drains active HTTP requests, then closes the pool.

Phase 1 accepts caller-provided prices in cents for a single implicit currency
(USD). There is no product catalog, authentication, creation idempotency, or event
publication yet. Each successful POST creates a new order; do not automatically
retry a timed-out POST. A timeout can occur after a committed write. Orders stay
`PENDING`, and no status mutation or cancellation API is exposed in this phase.

## Verify

```bash
go test -count=1 ./...
go vet ./...
go build -o bin/order-service ./cmd/order-service
# Requires a supported local C compiler for Go's race detector:
go test -race ./...

# Dedicated disposable integration database; never use a production database:
docker exec orderflow-postgres createdb -U orderflow orderflow_test
TEST_DATABASE_URL='postgres://orderflow:orderflow_local@localhost:5432/orderflow_test?sslmode=disable' \
  go test -count=1 -v ./tests
```

Without `TEST_DATABASE_URL`, the integration test explicitly skips; a passing
default test run does not prove PostgreSQL integration. The test applies the
bootstrap twice, verifies committed state through another pool, checks duplicate
primary-key rejection, and removes its test order. It leaves the schema in the
dedicated database. Migration 001 is an idempotent bootstrap, not a versioned
migration framework; future schema changes need new numbered migrations and
version tracking, rather than edits to an already applied file.

See [Phase 1 learning and manual checks](docs/phase-1.md), including persistence
and database failure experiments, and [validation record](docs/validation.md).
