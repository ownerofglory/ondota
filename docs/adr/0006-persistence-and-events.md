# ADR-0006: Persistence, events and infrastructure

- Status: accepted
- Date: 2026-10-03

## Context
The architecture should be event-driven without heavy brokers. billpiggy uses full event sourcing on PostgreSQL with a transactional outbox (`pkg/outbox`, `pkg/pgxtx`). ondOTA's state is small. The interesting history (enrollments, updates, rollbacks) is naturally a stream of events, but aggregates like `Tenant` or `Application` gain nothing from being rebuilt from events.

## Decision
- **PostgreSQL** is the only datastore. Migrations use golang-migrate (`migrations/NNNNNN_*.up/down.sql`) and run as a Helm hook job, as in billpiggy.
- **State tables + domain event log + outbox**, not full event sourcing:
  - Commands update state tables and append to `events.events` **in the same transaction** (`UnitOfWork` port, `pgxtx`-style context transaction).
  - `events.outbox` fans each event out per subscription. In-process consumers (desired-state recompute, update history, audit) are driven by a port of billpiggy's `pkg/outbox` engine: leases, backoff, dead-letter, per-aggregate ordering.
  - Device update history and the audit trail are projections of the event log.
- **Real-time push to devices** is out of scope. Devices poll `GET /device/v1/desired-state` with ETag and jitter (default 5 min). LISTEN/NOTIFY or SSE can come later without changing the model.
- **Deployment in namespace `ondota`:**
  - Postgres as a single StatefulSet (`postgres:17`, PVC on `local-path`). CloudNativePG is not used for now.
  - One server Deployment with two listeners (ADR-0003).
  - Traefik Ingresses for both hosts. cert-manager `letsencrypt-production` for the server certs.
  - Secrets: step-ca credentials, Google OAuth client, GitHub webhook secret, DB credentials. All are created from env, never committed.
- **Tests:**
  - Unit tests use in-memory adapters.
  - Integration tests take `ONDOTA_TEST_DATABASE_URL`: locally Postgres in `ondota` via `kubectl port-forward`, in CI a GitHub Actions `services: postgres` container.
  - step-ca is faked in tests with an in-process CA (`go.step.sm/crypto` `minica`) so CI doesn't depend on the cluster.

## Consequences
- Simpler than billpiggy's event sourcing, with the same outbox semantics, so code can be borrowed.
- If a context later needs full replay, its state table can be rebuilt from `events.events`, because every command records its event.
