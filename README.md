# MyMessaging

A real-time messaging platform built in Go and React, backed by RabbitMQ, PostgreSQL, and Valkey.

This project is being built in 5 progressive delivery stages. Currently at **Stage 1 (Foundation)**.

## Architecture & Tech Stack

- **API/WebSocket Gateway**: Go 1.22
- **Frontend**: React 18 + Vite, Zustand, TypeScript
- **Message Broker**: RabbitMQ 3.x (Topic exchanges + per-user durable queues)
- **Database**: PostgreSQL 16 (Cursor-paginated history)
- **Cache / Presence**: Valkey 8.x (Token blocklists + heartbeat)
- **Infrastructure**: Docker Compose

Read the full architecture and stage roadmap in [`docs/implementation_plan.md`](./docs/implementation_plan.md).

## Local Development (Docker Compose)

The entire integrated stack can be run with a single command:

```bash
# 1. Create your local .env file
cp .env.example .env

# 2. Build and start the cluster
docker compose up --build -d
```

### Services

| Service | Address | Description |
|---|---|---|
| **Web UI** | `http://localhost:5173` | React frontend application |
| **Gateway API** | `http://localhost:8080` | Go REST/WebSocket server |
| **PostgreSQL** | `localhost:5432` | Primary database |
| **Valkey** | `localhost:6379` | Cache/Presence |
| **RabbitMQ Admin**| `http://localhost:15672` | Management UI (guest/guest) |

## Development (Without Docker for hot-reloading)

If you just want to run the database/broker in Docker but natively run the code for hot reloading:

### 1. Start Infrastructure Only
```bash
docker compose up -d postgres valkey rabbitmq
```

### 2. Run the Gateway (Go)
```bash
cd services/gateway
go run ./cmd/main.go
```
*(The gateway auto-applies SQL migrations on boot)*

### 3. Run the Frontend (React Vite)
```bash
cd web
npm install
npm run dev
```

## Stage Roadmap

- [x] **Stage 1: Foundation** — Registration, Auth, real-time DMs (RabbitMQ bindings).
- [ ] **Stage 2: Reliability** — Offline delivery, read receipts, Dead-Letter Exchanges.
- [ ] **Stage 3: Groups** — Multi-user channels via topic fan-out bindings.
- [ ] **Stage 4: Presence & Search** — Valkey heartbeats, FTS, notifications.
- [ ] **Stage 5: Production Ready** — Rate limits, Admin dash, metrics, scaling out.
