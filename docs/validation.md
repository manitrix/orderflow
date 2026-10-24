# Phase 1 validation record

Executed on 2026-10-24 from Windows PowerShell, using Go 1.26.5 windows/amd64
and PostgreSQL 17 in Docker Desktop (engine 29.7.2).

| Check | Result |
| --- | --- |
| `go test -count=1 ./...` with `TEST_DATABASE_URL` set | PASS, including the PostgreSQL integration test |
| `go vet ./...` | PASS |
| `go build -o bin/order-service-verified.exe ./cmd/order-service` | PASS |
| Explicit executable `migrate` command | PASS |
| Live HTTP create/fetch/status | PASS: 201 creation, PENDING, computed total 3000 cents, matching fetched ID |
| Live health/readiness | PASS: 200 with database available |
| Database restart and read from a second application process | PASS: committed order preserved |
| Paused database | PASS: healthz 200; readyz 503 |
| Database unpaused | PASS: readiness and order fetch recovered |
| `go test -race ./...` | NOT RUN: Go reported that race requires CGO; this environment has no C compiler on PATH |
| OS signal shutdown/draining | IMPLEMENTED, not verified: Ctrl+C sent through the Windows tool PTY did not terminate the child process; verification processes were explicitly stopped during cleanup |

The integration test first exposed a test cleanup ordering bug (the connection
pool was closed before the cleanup callback); this was fixed and the final full
suite passed. The initial default test run skipped integration because its database
URL was unset; the final recorded run supplied the disposable test database URL.

Temporary test infrastructure used loopback ports 55432 (PostgreSQL), 18080 and
18081 (HTTP). The verification processes and auto-remove PostgreSQL container were
stopped after testing. Docker Desktop was started for verification and left running.
Downloaded Go dependencies, build outputs, and local cache files are gitignored.

No throughput or latency benchmark has been run. Kafka, Redis, inventory concurrency,
event duplication, and distributed crash recovery are future-phase tests. No
distributed reliability or Kubernetes behavior is claimed by this milestone.
