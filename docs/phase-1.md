# Phase 1: what to learn and check

The domain service validates requests independently of HTTP. A small repository
interface makes its business rules testable without PostgreSQL; the pgx adapter
implements the durable path. There is no general framework or generic repository.
Order items are JSONB because they are one aggregate saved/read together. Inventory
will later have normalized stock and reservation tables for concurrency control.

The insert is a single atomic database statement. Explicit multi-statement
transactions become necessary when writing an order and its outbox record together.
Application startup checks the schema but never applies DDL. Run `migrate` as a
separate deployment step. The initial migration itself runs in a transaction.

Every HTTP operation has a five-second context deadline. Startup checks have a
ten-second deadline, readiness checks one second, and shutdown ten seconds.
PostgreSQL connection establishment has a five-second timeout and the pool permits
ten connections. A context is cancellation, not a rollback guarantee after a commit.

Logs include correlation IDs, method, matched route, response status, and elapsed
time. Request bodies and the database URL are not logged. Liveness has no database
dependency; readiness does, so a database outage must not cause restart loops.

After starting with the README commands:

1. Create an order and save its ID. Restart the Go process; fetching it should
   return the same order, proving it is not in process memory.
2. Stop PostgreSQL with `docker stop orderflow-postgres`. `/healthz` should remain
   200 and `/readyz` should become 503. Data endpoints return a generic 500 on
   database errors and logs retain the diagnostic detail.
3. Use `docker start orderflow-postgres`, wait for readiness, and fetch the saved
   ID again. The named volume preserves data across container restarts.
4. Submit invalid quantities, duplicate product IDs, empty items, or malformed
   JSON. Expect 400 and no order creation. Try an unknown ID and expect 404.
5. Send Ctrl+C while running the service and inspect its JSON shutdown log.

These are manual experiments, not claims of tests already executed. No inventory,
Kafka, Redis, load, or Kubernetes experiment is part of this milestone.
