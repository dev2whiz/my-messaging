# Messaging Platform — Implementation Plan

A ground-up messaging platform allowing two or more registered users to exchange text messages (≤ 4,096 characters). Built stage-by-stage in Go, each stage delivering visible, testable value before the next.

---

## Confirmed Decisions

| Concern | Choice | Rationale |
|---|---|---|
| Backend language | **Go** | Strong concurrency model (goroutines), excellent AMQP/WebSocket libraries |
| Auth | **JWT (HS256)** | Stateless; Valkey-backed token blocklist for logout |
| Message broker | **RabbitMQ 3.x** | Best fit — see broker comparison below |
| Database | **PostgreSQL 16** | ACID, `tsvector` FTS, cursor pagination |
| Cache / Presence | **Valkey 8.x** | Open-source, license-clean Redis-compatible replacement (post Redis 7.4 SSPL) |
| Frontend | **React + Vite (TypeScript)** | Fast dev server, small bundle |
| Local dev | **Docker Compose** | All infra in one `compose up` |
| Roadmap tracking | **GitHub Issues + Milestones** | One milestone per stage; issues per feature/task |

> [!IMPORTANT]
> **Valkey** is the Linux Foundation fork of Redis, fully API-compatible with Redis 7.x. Valkey 8.x is used in place of Redis throughout this project to avoid the SSPL license introduced in Redis 7.4.

---

## Message Broker: Why RabbitMQ

| Broker | Persistence | Fan-out | Ops Complexity | Fit |
|---|---|---|---|---|
| **RabbitMQ** | Durable queues | Topic exchanges | Low–Medium | ✅ Best |
| Apache Pulsar | Always-on | Topics | High (ZooKeeper/BookKeeper) | Overkill |
| Redis/Valkey Pub/Sub | ❌ None | Channels | Very Low | No delivery guarantee |
| Apache Kafka | Always-on (log) | Consumer groups | High | Overkill |

**RabbitMQ** maps one durable queue per user (`user.<id>`), fan-out to groups via topic bindings, and provides Dead-Letter Exchanges (DLX) for undeliverable messages — all with a single Docker image.

---

## High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                         Clients                               │
│          Browser (React/Vite)     |    Mobile (future)        │
└────────────────┬──────────────────────────┬──────────────────┘
                 │ HTTPS / WebSocket        │
         ┌───────▼──────────────────────────▼──────┐
         │            API Gateway (Go)               │
         │  REST:  auth, user mgmt, message history  │
         │  WS:    real-time message delivery         │
         └──────────┬────────────────┬───────────────┘
                    │ AMQP           │ SQL / Valkey
         ┌──────────▼──────┐  ┌─────▼───────────────┐
         │   RabbitMQ      │  │  PostgreSQL + Valkey  │
         │  (message bus)  │  │  (store + presence)   │
         └─────────────────┘  └─────────────────────┘
```

**Message flow (DM):**
1. Sender posts via REST or WebSocket frame → Gateway validates JWT, enforces 4 KB limit.
2. Gateway persists message to Postgres, publishes to RabbitMQ exchange `messaging` routing key `user.<recipient_id>`.
3. Consumer bound to `user.<recipient_id>` queue delivers via WebSocket if recipient is online; otherwise held until reconnect.

---

## Directory Structure

```
my-messaging/
├── docker-compose.yml            # postgres, valkey, rabbitmq, gateway, web
├── .env.example
├── migrations/                   # SQL files (golang-migrate)
│   ├── 001_create_users.up.sql
│   ├── 002_create_conversations.up.sql
│   └── 003_create_messages.up.sql
├── services/
│   └── gateway/                  # Go service
│       ├── cmd/
│       │   └── main.go
│       ├── internal/
│       │   ├── auth/             # JWT issue / validate / blocklist (Valkey)
│       │   ├── broker/           # RabbitMQ publisher & consumer
│       │   ├── handler/          # HTTP + WebSocket handlers
│       │   ├── middleware/       # Auth, rate-limit, logging
│       │   ├── model/            # User, Message, Conversation structs
│       │   └── store/            # Postgres queries (sqlx)
│       ├── Dockerfile
│       └── go.mod
├── web/                          # React/Vite frontend
│   ├── src/
│   │   ├── api/                  # REST client + WebSocket hook
│   │   ├── components/           # MessageList, MessageInput, UserSidebar
│   │   ├── pages/                # Login, Register, Chat
│   │   └── store/                # Zustand state (auth, conversations)
│   ├── index.html
│   └── package.json
└── docs/
    ├── ARCHITECTURE.md
    └── API.md
```

---

## Stage-by-Stage Delivery Plan

### Stage 1 — Foundation *(current)*
**Milestone**: `v0.1.0 — Foundation`

> Register, login, and exchange real-time direct messages between two users.

#### Backend tasks (GitHub issues)
- [ ] `#1` — Docker Compose: Postgres, Valkey, RabbitMQ, gateway, web services
- [ ] `#2` — DB migrations: `users`, `conversations`, `conversation_members`, `messages`
- [ ] `#3` — `POST /auth/register` — bcrypt password hash, return JWT
- [ ] `#4` — `POST /auth/login` — verify credentials, return JWT; `POST /auth/logout` (Valkey blocklist)
- [ ] `#5` — JWT middleware (validate token, check Valkey blocklist)
- [ ] `#6` — RabbitMQ setup: topic exchange `messaging`, auto-declare durable queue `user.<id>` on register
- [ ] `#7` — `POST /messages` — validate body ≤ 4096 chars, persist, publish to RabbitMQ
- [ ] `#8` — WebSocket handler `/ws` — auth via JWT query param, bridge queue → WS frame, heartbeat ping/pong
- [ ] `#9` — `GET /conversations/:id/messages` — paginated history (cursor-based)
- [ ] `#10` — `GET /users` — list registered users (to start a new DM)

#### Frontend tasks (GitHub issues)
- [ ] `#11` — Project scaffold (Vite + React + TypeScript, Zustand, React Router)
- [ ] `#12` — Register & Login pages (JWT stored in `httpOnly` cookie or memory)
- [ ] `#13` — Chat page: sidebar (user list), message thread, input box
- [ ] `#14` — `useWebSocket` hook — connect, receive frames, auto-reconnect
- [ ] `#15` — Send message (REST `POST /messages`), optimistic UI update
- [ ] `#16` — Scroll-to-bottom, load earlier messages on scroll-up

#### Stage 1 Verification
```bash
# Unit tests
cd services/gateway && go test ./... -v

# Integration (Docker Compose)
docker compose up -d
go test ./integration/... -v

# Manual: open two browser tabs, register two users, chat
```

**Done-criteria**: Two browser tabs can register, log in, send and receive messages in real time with no page reload.

---

### Stage 2 — Reliability
**Milestone**: `v0.2.0 — Reliability`

- Offline delivery (durable queues + publisher confirms)
- Consumer ACKs before queue removal
- Dead-Letter Exchange for failed deliveries
- Read receipts (`messages.read_at`); delivered status (✓✓)
- Cursor-paginated history improved UX

---

### Stage 3 — Group Chats
**Milestone**: `v0.3.0 — Groups`

- Create / manage group conversations
- Fan-out via RabbitMQ topic binding: `group.<id>` → all member queues
- Typing indicators (direct WS broadcast, no broker hop)
- Group admin role

---

### Stage 4 — Presence, Notifications & Search
**Milestone**: `v0.4.0 — Presence & Search`

- Valkey presence heartbeat (`SETEX user:online:<id> 30 1`)
- `GET /users/:id/status` online/offline
- Browser Push / email notification for offline users
- Full-text search (`tsvector`) on message body
- Emoji reactions (JSONB + WS broadcast)

---

### Stage 5 — Admin, Observability & Hardening
**Milestone**: `v1.0.0 — Production Ready`

- Admin dashboard (user management, DLX queue depth, broker proxy)
- Token-bucket rate limiting (Valkey) — 60 msg/min per user → `429`
- Prometheus `/metrics` + bundled Grafana dashboard
- Compliance audit log (append-only Postgres table)
- Horizontal gateway scaling (stateless service; Valkey shared session/presence)
- E2E TLS, secret management via env / Docker secrets

---

## GitHub Project Setup

```
Repository:  github.com/<org>/my-messaging
Milestones:
  v0.1.0 — Foundation    (Stage 1)
  v0.2.0 — Reliability   (Stage 2)
  v0.3.0 — Groups        (Stage 3)
  v0.4.0 — Presence & Search (Stage 4)
  v1.0.0 — Production Ready  (Stage 5)

Labels:
  type:feature  type:bug  type:chore  type:docs
  stage:1  stage:2  stage:3  stage:4  stage:5
  component:backend  component:frontend  component:infra

Branch strategy:
  main          → stable, tagged releases
  develop       → integration branch
  feat/<issue>  → per-issue feature branches
  PR required for merge to develop; squash merge to main on release
```

> [!NOTE]
> Issues `#1`–`#16` above map directly to Stage 1 milestone. Each subsequent stage will have its own numbered issue list created at the start of that stage.

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| Valkey 8.x over Redis | License-clean (BSD-3); 100% API-compatible drop-in; LF governance |
| One durable queue per user | Simple fan-out model; scales with RabbitMQ clustering |
| JWT + Valkey blocklist | Stateless validation; logout invalidation without DB round-trip |
| Postgres for persistence | ACID guarantees; native FTS with `tsvector` |
| Cursor pagination (`id < cursor`) | Stable under concurrent writes vs. `OFFSET` |
| WebSocket over SSE | Bidirectional: needed for ACKs, typing indicators, reactions |
| httpOnly cookie for JWT | XSS-safe token storage (alternative: in-memory only) |
