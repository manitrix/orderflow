# Milestones

Work stops at the end of each phase for review and approval.

| Phase | Deliverable | Verification gate |
| --- | --- | --- |
| 1 Foundation | Order REST API, PostgreSQL bootstrap, validation, JSON logs, shutdown | Unit/API tests, build/vet, database round trip when infrastructure is available |
| 2 Containers | Non-root multi-stage Order image, Compose PostgreSQL, health checks, volume | Create/fetch across application and database restart; container DNS |
| 3 Events | KRaft Kafka, Inventory, simulated Payment, separate schema roles, Redis limiter | Full success, stock rejection, payment failure and compensation flows |
| 4 Reliability | Outboxes for all writers, inbox deduplication, creation idempotency, retries, DLQ | Crash replay, duplicate delivery, concurrent last-item reservation, outage recovery |
| 5 Kubernetes | kind, Deployments, Services, config/secrets templates, probes, resources, HPA/Ingress examples | DNS, restart recovery, replicas, rollout and debugging exercises |
| 6 Observability | Prometheus request/error metrics, consumer lag, k6 scenarios | Measured load report with environment and bottlenecks; no invented numbers |

Phase 3 establishes event routing, but must be labeled crash-unsafe until Phase 4
closes the database/Kafka dual-write gap. No phase claims production readiness.
Kubernetes lessons will first use a single-node kind cluster and explain external
Compose endpoints (including Kafka advertised addresses), then multi-node kind and
the storage/backup/operator trade-offs for stateful infrastructure.
