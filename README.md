# MyMessaging

A real-time messaging platform built with Spring Boot and React, backed by RabbitMQ, PostgreSQL, and Valkey.

This project is being built in 5 progressive delivery stages. Currently at **Stage 1 (Foundation) — complete**.

## Architecture & Tech Stack

| Layer | Technology |
|---|---|
| **API / WebSocket Gateway** | Spring Boot 3.5, Java 21 (Virtual Threads) |
| **Frontend** | React 18, Vite, TypeScript, Zustand |
| **Message Broker** | RabbitMQ 3.x — Topic exchange, per-user durable queues, manual AMQP ack |
| **Database** | PostgreSQL 16 — cursor-paginated message history |
| **Cache / Token blocklist** | Valkey 8.x — JWT JTI blocklist with TTL-parity |
| **Infrastructure** | Docker Compose — fully containerised stack |

Read the current architecture and migration roadmap in [`docs/00-implementation-plan-01.md`](./docs/00-implementation-plan-01.md).

For historical context, the original Go-stage implementation plan is in [`docs/00-implementation-plan-00.md`](./docs/00-implementation-plan-00.md).

## Environment Configuration

The gateway reads configuration from environment variables. Copy the example file before starting:

```bash
cp .env.example .env
```

Key variables (all have safe defaults for local development):

| Variable | Default | Description |
|---|---|---|
| `JWT_SECRET` | `supersecretkeythatisatleast32byteslong` | HMAC-SHA signing key — **change in production** |
| `JWT_EXPIRATION` | `86400000` | Token lifetime in milliseconds (24 h) |
| `ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:4173` | Comma-separated CORS / WS origin allowlist |
| `POSTGRES_*` | see docker-compose.yml | Database connection |
| `RABBITMQ_*` | see docker-compose.yml | Broker connection |
| `VALKEY_HOST/PORT` | `localhost:6379` | Cache connection |

## Local Development (Docker Compose)

The entire stack — gateway, frontend, database, broker, and cache — runs with a single command:

```bash
# 1. Configure environment
cp .env.example .env   # edit JWT_SECRET and ALLOWED_ORIGINS for non-local use

# 2. Build and start the cluster
docker compose up --build -d
```

Migrations are applied automatically by the `migrate` container before the gateway starts.

### Services

| Service | Address | Description |
|---|---|---|
| **Web UI** | `http://localhost:5173` | React frontend |
| **Gateway API** | `http://localhost:8080` | Spring Boot REST + WebSocket (`/ws`) |
| **PostgreSQL** | `localhost:5432` | Primary database |
| **Valkey** | `localhost:6379` | JWT blocklist cache |
| **RabbitMQ Admin** | `http://localhost:15672` | Management UI (`guest` / `guest`) |

## Development Without Docker (Hot-Reload)

Run infrastructure in Docker and services natively for fast iteration:

### 1. Start Infrastructure Only
```bash
docker compose up -d postgres valkey rabbitmq
docker compose up migrate   # runs once then exits
```

### 2. Run the Gateway
```bash
cd services/gateway-spring
mvn spring-boot:run
```

### 3. Run the Frontend
```bash
cd web
npm install
npm run dev
```

The Vite dev server proxies `/api` → `http://localhost:8080` and `/ws` → `ws://localhost:8080` automatically (see `vite.config.ts`).

## Supported Commands

The repo uses a **root Makefile** as the main command surface. The root Makefile delegates to module-specific Makefiles in `services/gateway-spring/` and `web/`, while reusable automation logic lives under `scripts/`.

### Frequent Tasks

| Command | Purpose |
|---|---|
| `make help` | Show all supported targets |
| `make up` | Docker: build and start the full stack |
| `make down` | Docker: stop the full stack |
| `make restart` | Docker: rebuild and restart the full stack |
| `make ps` | Docker: show Compose service status |
| `make logs` | Docker: tail Compose logs |
| `make infra-up` | Docker: start only Postgres, Valkey, and RabbitMQ |
| `make infra-down` | Docker: stop only Postgres, Valkey, and RabbitMQ |
| `make infra-clean` | Docker: destroy containers, volumes, networks, and ephemeral state |
| `make migrate` | Docker: run the migrations container once |
| `make gateway-run` | Local process: run the Spring Boot gateway |
| `make gateway-test` | Local process: run Spring Boot unit tests |
| `make gateway-build` | Local process: build the Spring Boot artifact |
| `make web-install` | Local process: install frontend dependencies |
| `make web-run` | Local process: run the Vite dev server |
| `make web-build` | Local process: build the frontend bundle |
| `make build` | Local process: build backend and frontend |
| `make test` | Local process: run backend tests and frontend build check |
| `make smoke` | Docker-backed integration smoke test |
| `make smoke-local` | Local app processes + Docker infra smoke test |
| `make check` | Local tests followed by Docker smoke test |
| `make clean` | Local process: clean backend and frontend build outputs |

`make infra-clean` is destructive. It removes Docker Compose containers, named volumes, and networks for this project, wiping the database, broker state, and cache contents.

### Recommended Workflows

#### Fast local development
```bash
make infra-up
make migrate

# terminal 1
make gateway-run

# terminal 2
make web-run
```

#### Full integration validation
```bash
make up
make smoke
```

#### Pre-merge sanity run
```bash
make check
```

#### Reset infrastructure state
```bash
make infra-clean
make infra-up
make migrate
```

## Key API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/auth/register` | — | Register with username, email, password |
| POST | `/auth/login` | — | Login with email, password → JWT |
| POST | `/auth/logout` | Bearer | Invalidate token (JTI-keyed blocklist) |
| DELETE | `/auth/me` | Bearer | Unregister current user and remove Stage 1 account data |
| GET | `/auth/me` | Bearer | Current user profile |
| GET | `/users` | Bearer | List all users |
| POST | `/messages` | Bearer | Send a DM (`recipient_id`, `body`) |
| GET | `/conversations/{id}/messages` | Bearer | Paginated message history |
| GET | `/conversations/direct/{partnerId}` | Bearer | Resolve DM conversation |
| WS | `/ws?token=<jwt>` | Query JWT | Real-time message stream |

## Notable Implementation Details

- **Real-time delivery**: Each authenticated user gets a durable RabbitMQ queue (`user.<uuid>`). Queues are declared idempotently on registration, login, and WS connect — surviving broker restarts.
- **Manual AMQP ack**: The WS handler acks only after a successful socket send; nacks with requeue on closed session or error — no silent message loss.
- **WS reconnect**: The frontend hook uses exponential backoff with jitter, an `online` event listener for network-restore reconnect, and a post-reconnect history sync to fill any gap.
- **JWT security**: Tokens include a `jti` UUID claim. Logout stores `jwt:blocklist:<jti>` in Valkey with TTL matching token expiry — constant 36-char key size regardless of token length.
- **CORS / WS origins**: Explicit allowlist via `ALLOWED_ORIGINS` env var; no wildcards in production.
- **Token transport**: `?token=` query parameter only accepted on `/ws` upgrade path; all HTTP endpoints require `Authorization: Bearer`.
- **Smoke-test hygiene**: `scripts/smoke-test.sh` now calls `DELETE /auth/me` for its temporary users after each run, so sanity checks do not pollute the database or reach into Postgres directly.

## Deprecated Gateway

The original Go gateway is preserved at `services/gateway-go-deprecated/` for reference only. It is not built or started by Docker Compose.

## Stage Roadmap

- [x] **Stage 1: Foundation** — Registration, Auth (JTI revocation), real-time DMs, WS reconnect hardening, connection badge UI.
- [ ] **Stage 2: Reliability** — Dead-Letter Exchanges, offline delivery queue, read receipts.
- [ ] **Stage 3: Groups** — Multi-user channels via topic fan-out bindings.
- [ ] **Stage 4: Presence & Search** — Valkey heartbeats, full-text search, push notifications.
- [ ] **Stage 5: Production Ready** — CI workflow, release automation, rate limits, admin dashboard, metrics, horizontal scaling.
