# Messaging Platform — Implementation Stages Plan

This document is the single source of truth for architecture direction and stage-by-stage delivery.

It consolidates the earlier:
- `00-implementation-plan-00.md` (original Go-first staged plan)
- `00-implementation-plan-01.md` (Spring Boot migration plan)

The active runtime is now **Spring Boot**. The earlier Go gateway implementation is retained in the repository as **deprecated reference code** at `services/gateway-go-deprecated/` and is not removed yet.

---

## Current Stage Status

- **Stage 1 (Foundation): COMPLETE**
- **Stage 2 (Reliability): READY TO START**

Stage 1 completion is based on:
- end-to-end direct messaging flow working
- WebSocket reliability hardening in place
- smoke sanity automation available (`scripts/smoke-test.sh`)
- backend/frontend build validation and passing test suite

---

## Confirmed Decisions

| Concern | Choice | Rationale |
|---|---|---|
| Backend language/runtime | **Java 21 + Spring Boot 3.x** | Production-ready ecosystem, virtual-thread concurrency, strong testability |
| Auth | **JWT (HS256)** | Stateless auth, Valkey-backed token revocation (`jti` blocklist) |
| Message broker | **RabbitMQ 3.x** | Durable queue model + topic routing + DLX-ready for Stage 2 |
| Database | **PostgreSQL 16** | ACID persistence, reliable relational modeling, cursor pagination |
| Cache / Presence | **Valkey 8.x** | Redis-compatible, open governance, token blocklist support |
| Frontend | **React + Vite (TypeScript)** | Fast iteration, lightweight runtime |
| Local orchestration | **Docker Compose** | Reproducible multi-service environment |
| Command surface | **Root Makefile delegating to modules** | Consistent workflow across backend/frontend/infra |
| CI introduction | **Stage 5** | Keep early stages lean; add enforceable quality gates in production-hardening stage |

> [!IMPORTANT]
> Valkey is used as a Redis-compatible cache/presence layer. RabbitMQ remains the durable messaging broker.

---

## Message Broker Choice: RabbitMQ

| Broker | Persistence | Fan-out | Ops Complexity | Fit |
|---|---|---|---|---|
| **RabbitMQ** | Durable queues | Topic exchanges | Low–Medium | ✅ Best |
| Apache Pulsar | Always-on | Topics | High | Overkill |
| Redis/Valkey Pub/Sub | No durable delivery | Channels | Very Low | Not enough guarantees |
| Apache Kafka | Durable log | Consumer groups | High | Overkill for current scope |

RabbitMQ maps naturally to per-user queues (`user.<id>`) and supports Stage 2 DLX expansion.

---

## High-Level Architecture (Current)

```
┌──────────────────────────────────────────────────────────────┐
│                         Clients                              │
│          Browser (React/Vite)     |    Mobile (future)       │
└────────────────┬──────────────────────────┬──────────────────┘
                 │ HTTPS / WebSocket        │
         ┌───────▼──────────────────────────▼──────┐
         │      API Gateway (Spring Boot 3.x)      │
         │  REST: auth, users, history, messages   │
         │  WS:   real-time message delivery       │
         └──────────┬───────────────┬──────────────┘
                    │ AMQP          │ SQL / Valkey
         ┌──────────▼──────┐  ┌─────▼───────────────┐
         │   RabbitMQ      │  │ PostgreSQL + Valkey │
         │  (message bus)  │  │ (store + blocklist) │
         └─────────────────┘  └─────────────────────┘
```

Message flow (DM):
1. Sender submits message via REST.
2. Gateway validates auth and payload, persists in Postgres.
3. Gateway publishes to RabbitMQ exchange `messaging` with `user.<recipient_id>` routing key.
4. Recipient WebSocket listener forwards delivery frame in real time.
5. History endpoint serves cursor-paginated retrieval from Postgres.

---

## Current Directory Structure

```
my-messaging/
├── docker-compose.yml
├── Makefile
├── .env.example
├── migrations/
│   ├── 001_create_users.*.sql
│   ├── 002_create_conversations.*.sql
│   ├── 003_create_messages.*.sql
│   └── 004_fix_conversation_created_by.*.sql
├── scripts/
│   └── smoke-test.sh
├── services/
│   ├── gateway-spring/
│   │   ├── Makefile
│   │   ├── pom.xml
│   │   └── src/
│   └── gateway-go-deprecated/     # deprecated reference implementation
├── web/
│   ├── Makefile
│   ├── package.json
│   └── src/
└── docs/
    ├── 00-implementation-stages-plan.md
    ├── 01-stage-walkthrough.md
    └── 01-stage-review-n-fixes.md
```

---

## Stage-by-Stage Delivery Plan

### Stage 1 — Foundation (COMPLETE)
Milestone: `v0.1.0 — Foundation`

Delivered outcomes:
- registration/login/logout/auth-me
- JWT revocation (`jti`) via Valkey blocklist
- per-user RabbitMQ durable queue declaration + publish
- direct messaging + history retrieval
- WebSocket delivery and reconnect handling
- security and contract fixes from Stage 1 remediation backlog
- smoke automation + Make-driven local workflows

Done criteria: met.

### Stage 2 — Reliability (NEXT)
Milestone: `v0.2.0 — Reliability`

Planned scope:
- offline delivery behavior hardening
- explicit delivery semantics and failure paths
- Dead-Letter Exchange / DLQ handling for undeliverables
- read receipts (`read_at`) and delivered indicators
- stronger integration tests around disconnect/reconnect scenarios

### Stage 2 Kickoff Checklist
| # | Kickoff Task | Acceptance Criteria |
|---|---|---|
| 1 | Confirm baseline branch and environment parity | `make test` (backend + frontend) and `make smoke` pass on the latest Stage 1 baseline before any Stage 2 code is merged |
| 2 | Finalize reliability data-model changes | Stage 2 migration draft reviewed for `read_at` and delivery-state fields, required indexes, and rollback scripts |
| 3 | Implement reliability migrations | New migration files apply and rollback cleanly on a fresh database |
| 4 | Add DLX/DLQ broker topology | RabbitMQ exchange/queue/binding config supports dead-letter routing and is validated by integration test |
| 5 | Harden offline delivery behavior | Recipient disconnect/reconnect flow preserves queued deliveries with no loss in tested scenarios |
| 6 | Add explicit delivery semantics | Delivery success/failure paths are codified (ack/nack/requeue policy) and covered by tests |
| 7 | Implement read receipts | Read endpoint/event updates `read_at` correctly and is reflected in API/WS payloads |
| 8 | Expand reliability-focused tests | Integration tests cover reconnect storms, duplicate delivery prevention, and DLQ routing |
| 9 | Preserve frontend compatibility | Frontend builds and existing Stage 1 chat/auth flows remain regression-free with Stage 2 backend changes |
| 10 | Publish Stage 2 verification record | `docs/01-stage-walkthrough.md`, `README.md`, and smoke documentation updated with Stage 2 checks and outcomes |

### Stage 3 — Group Chats
Milestone: `v0.3.0 — Groups`

Planned scope:
- group conversation management
- fan-out routing via topic bindings (`group.<id>`)
- typing indicators and group moderation controls

### Stage 4 — Presence, Notifications & Search
Milestone: `v0.4.0 — Presence & Search`

Planned scope:
- presence heartbeat/status model
- offline notifications
- full-text search on messages
- richer message metadata (for example reactions)

### Stage 5 — Production Ready
Milestone: `v1.0.0 — Production Ready`

Planned scope:
- CI workflow introduction (enforced quality gates)
- release automation and image publishing
- metrics/observability stack
- rate limiting and operational hardening
- deployment promotion workflow and rollout validation

---

## Delivery Workflow By Stage

### Stage 1-4 Command Strategy
- Use root `Makefile` as primary command surface.
- Delegate module tasks to:
  - `services/gateway-spring/Makefile`
  - `web/Makefile`
- Keep task logic in scripts (for example `scripts/smoke-test.sh`).

Typical commands:
- `make infra-up`, `make migrate`
- `make gateway-run`, `make web-run`
- `make build`, `make test`
- `make smoke`, `make check`

### Stage 5 Production Additions
- Introduce CI workflow (not before Stage 5).
- Minimum CI gates:
  - backend tests
  - frontend build/type checks
  - docker-based smoke integration test
- Add artifact/image publication and release governance.

---

## Deprecated Go Implementation Note

The previous Go gateway implementation remains in `services/gateway-go-deprecated/` for historical reference and migration traceability.

- It is **deprecated**.
- It is **not** the active runtime path.
- It may be removed in a future cleanup stage after explicit sign-off.

---

## Key Design Decisions (Still Applicable)

| Decision | Rationale |
|---|---|
| Per-user durable RabbitMQ queues | Simple direct-message routing model, resilient delivery path |
| JWT + Valkey revocation | Stateless auth with logout invalidation |
| Postgres as system of record | Strong consistency and relational integrity |
| Cursor pagination for history | Stable pagination under concurrent writes |
| WebSocket for real-time delivery | Low-latency push suitable for chat UX |
