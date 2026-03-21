# Stage 1 Verification & Walkthrough

**Status: ✅ CLOSED — 21 March 2026**

All planned Stage 1 deliverables and the full P0/P1 remediation backlog have been completed and verified. See [`01-stage-review-n-fixes.md`](./01-stage-review-n-fixes.md) for the itemised fix record.

---

## What Was Built

Stage 1 delivers the **Foundation (v0.1.0)** of the messaging platform. The active runtime is the **Spring Boot gateway** (`services/gateway-spring`). The original Go gateway has been moved to `services/gateway-go-deprecated` and is preserved for reference only.

### Infrastructure — `docker-compose.yml`
Orchestrates five services with health-checked startup ordering:

| Service | Image | Port(s) |
|---|---|---|
| postgres | postgres:16-alpine | 5432 |
| valkey | valkey/valkey:8-alpine | 6379 |
| rabbitmq | rabbitmq:3-management-alpine | 5672, 15672 |
| migrate | migrate/migrate:v4.18.1 | — (runs once, then exits) |
| gateway | ./services/gateway-spring | 8080 |
| web | ./web | 5173→80 |

The `migrate` container applies all SQL migrations before the gateway starts. The `gateway` container waits on `migrate: service_completed_successfully`, `valkey: healthy`, and `rabbitmq: healthy`.

### Database Migrations (`migrations/`)
| File | Purpose |
|---|---|
| `001_create_users.up.sql` | `users` table — UUID PK, bcrypt password, email unique index |
| `002_create_conversations.up.sql` | `conversations` + `conversation_members` tables |
| `003_create_messages.up.sql` | `messages` table — cursor index on `(conversation_id, sent_at DESC)` |
| `004_fix_conversation_created_by.up.sql` | Drops `NOT NULL` on `conversations.created_by` to allow `ON DELETE SET NULL` |

### Spring Boot Gateway (`services/gateway-spring`)
**Auth layer** (`AuthController`)
- `POST /auth/register` — creates user, bcrypt-hashes password, declares RabbitMQ queue, returns JWT + `UserDto`
- `POST /auth/login` — looks up by email, re-declares queue (idempotent recover after broker restart), returns JWT + `UserDto`
- `POST /auth/logout` — invalidates token via JTI-keyed Valkey blocklist with TTL parity
- `GET /auth/me` — returns `UserDto` for the authenticated principal
- `DELETE /auth/me` — unregisters current user and removes Stage 1 data traces (used by smoke cleanup)

**Messaging layer** (`MessageController`)
- `POST /messages` — validates recipient, resolves or creates DM conversation, saves message, publishes to RabbitMQ routing key `user.<recipient-uuid>`, returns `MessageWithSender`
- `GET /conversations/{id}/messages` — cursor-paginated history, joined with sender username
- `GET /conversations/direct/{partnerId}` — resolves existing DM conversation

**User layer** (`UserController`)
- `GET /users` — lists all users as `UserDto` (no password fields)
- `GET /users/me` — alias for current user

**Security** (`JwtAuthenticationFilter`, `JwtTokenProvider`, `SecurityConfig`)
- Tokens include a `jti` UUID claim; blocklist key is `jwt:blocklist:<jti>` (36-char constant size)
- `?token=` query parameter only extracted when `requestURI == /ws` — all other HTTP paths require `Authorization: Bearer`
- CORS and WS origin allowlist driven by `app.allowed-origins` / `ALLOWED_ORIGINS` env var; no wildcards

**WebSocket** (`MessagingWebSocketHandler`)
- Authenticated via `?token=<jwt>` on upgrade
- Per-session `SimpleMessageListenerContainer` consuming from `user.<uuid>` queue
- Queue + binding declared idempotently at connect time
- `AcknowledgeMode.MANUAL` — `basicAck` on successful WS send, `basicNack` with requeue on closed session or error
- `handleTransportError` override for clean logging

**Broker** (`RabbitBrokerService`, `RabbitConfig`)
- Topic exchange `messaging`, durable per-user queues `user.<uuid>`
- `Jackson2JsonMessageConverter` with `JavaTimeModule` + `WRITE_DATES_AS_TIMESTAMPS=false` — all timestamps serialised as ISO-8601 strings

### React Frontend (`web/`)
| File | Purpose |
|---|---|
| `LoginPage.tsx` | Email + password sign-in |
| `RegisterPage.tsx` | Username + email + password registration |
| `ChatPage.tsx` | Sidebar user list, message thread, send textarea, char counter |
| `useWebSocket.ts` | WS lifecycle hook — exponential backoff with jitter, `online` event reconnect, post-reconnect history sync, `WsConnectionState` export |
| `authStore.ts` | Zustand persist store — token + user |
| `api/client.ts` | Typed fetch wrapper, reads token from localStorage |

**Connection badge** — the chat header shows a live pill indicator: `Live` (green) / `Connecting` / `Reconnecting` (amber) / `Offline` (red), driven by `WsConnectionState` returned from `useWebSocket`.

---

## Verification Results

### Unit Tests (Spring Boot)
```
Tests run: 52, Failures: 0, Errors: 0, Skipped: 0
BUILD SUCCESS
```
Test coverage spans: `JwtTokenProvider`, `JwtAuthenticationFilter`, `SecurityConfig`, `RabbitConfig`, `RabbitBrokerService`, `AuthController`, `MessageController`, `UserController`, `ModelMapping`, `MessagingWebSocketHandler`.

### Frontend Build
```
vite v5.4.21 — 355 modules transformed
dist/assets/index.css   8.54 kB
dist/assets/index.js  193.00 kB
✓ built in ~1s — no TypeScript errors
```

### API Smoke Test

**Register:**
```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"dev1","email":"dev1@test.local","password":"password123"}'
```
```json
{
  "token": "eyJhb...<jwt>",
  "user": {
    "id": "47a732ab-2869-48b3-88fd-f0611ec8d7bd",
    "username": "dev1",
    "email": "dev1@test.local",
    "created_at": "2026-03-21T10:00:00.000Z",
    "updated_at": "2026-03-21T10:00:00.000Z"
  }
}
```
No `password` field in response. Timestamps in ISO-8601. RabbitMQ queue declared.

**Login:**
```bash
curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"dev1@test.local","password":"password123"}'
```
Returns same `AuthResponse` shape. Queue re-declared idempotently.

**Send message:**
```bash
curl -s -X POST http://localhost:8080/messages \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"recipient_id":"<uuid>","body":"Hello!"}'
```
```json
{
  "id": "...",
  "conversation_id": "...",
  "sender_id": "...",
  "sender_username": "dev1",
  "body": "Hello!",
  "sent_at": "2026-03-21T10:01:00.000Z",
  "read_at": null
}
```
Message simultaneously written to DB and published to recipient's RabbitMQ queue. Live WS delivery confirmed.

---

## Stage 1 Checklist

| # | Deliverable | Status |
|---|---|---|
| 1 | Docker Compose — all services healthy with ordered startup | ✅ |
| 2 | SQL migrations — users, conversations, members, messages | ✅ |
| 3 | User registration with bcrypt, JWT issuance, queue declaration | ✅ |
| 4 | Email-based login, queue re-declaration on login | ✅ |
| 5 | JWT logout with JTI-keyed Valkey blocklist | ✅ |
| 6 | Password excluded from all API responses (DTO boundary) | ✅ |
| 7 | Send DM — persists + publishes to RabbitMQ per-user queue | ✅ |
| 8 | Cursor-paginated message history with sender username | ✅ |
| 9 | WebSocket bridge — AMQP queue → WS session, manual ack | ✅ |
| 10 | WS reconnect — backoff, jitter, online-event, history sync | ✅ |
| 11 | ISO-8601 timestamps on WS frames (no "56 years ago" bug) | ✅ |
| 12 | CORS / WS origin allowlist via env var (no wildcards) | ✅ |
| 13 | Query-param token restricted to `/ws` path only | ✅ |
| 14 | JTI-based JWT revocation (bounded key size in Redis/Valkey) | ✅ |
| 15 | `is_group = FALSE` filter on DM conversation lookup | ✅ |
| 16 | Connection badge UI (Live / Connecting / Reconnecting / Offline) | ✅ |
| 17 | API-based smoke cleanup via `DELETE /auth/me` (no direct SQL in script) | ✅ |
| 18 | 52 unit tests passing, frontend build clean | ✅ |

---

## Stage 2 Preview

The next stage targets reliability and delivery guarantees:
- **Dead-Letter Exchange (DLX)** — undeliverable messages routed to a DLQ for inspection
- **Offline delivery** — messages queued while recipient is disconnected, flushed on reconnect
- **Read receipts** — `read_at` timestamp set when recipient opens the thread
- **P2-1** Global JSON naming consistency (snake_case enforcement via `@JsonNaming`)
- **P2-2** Maven dependency security scan (Dependabot / OWASP plugin)
- **P2-3** Documentation refresh (this file serves as the close record for Stage 1)
