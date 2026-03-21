# Smoke Test Guide

This guide validates core Stage 1 integration behavior with a fast automated sanity run.

## Status Review

The previous manual smoke steps were mostly valid, but one request field was outdated:
- `POST /messages` payload must use `recipient_id` (snake_case), not `recipientId`.

The recommended path is now automation via `scripts/smoke-test.sh`.

## Automated Smoke Test (Recommended)

### Prerequisites

- `curl`
- `jq`
- `bash`
- Docker (for Docker mode)

### 1. Docker Mode (full container stack)

```bash
docker compose up --build -d
./scripts/smoke-test.sh docker
```

What this validates:
- UI reachable (`http://localhost:5173`)
- Register A (direct gateway)
- Login A (email contract)
- Authenticated `/auth/me`
- Authenticated `/users`
- Register B through web proxy (`/api`)
- Send message A -> B with `recipient_id`
- Resolve direct conversation + fetch message history
- Cleanup via authenticated `DELETE /auth/me` for the temporary smoke-test users
- Response payload shape checks via `jq`

### 2. Local Mode (infra in Docker, app processes local)

Run infra and apps locally (example):

```bash
docker compose up -d postgres valkey rabbitmq
docker compose up migrate

# terminal 1
cd services/gateway-spring
mvn spring-boot:run

# terminal 2
cd web
npm run dev
```

Then run:

```bash
./scripts/smoke-test.sh local
```

Notes:
- Default URLs still assume `http://localhost:5173` and `http://localhost:8080`.
- For custom ports/hosts, override env vars:

```bash
WEB_URL=http://localhost:5174 \
API_URL=http://localhost:8081 \
PROXY_API_URL=http://localhost:5174/api \
./scripts/smoke-test.sh local
```

## Output and Failure Handling

- On success: script prints `[smoke] SUCCESS: all sanity checks passed`.
- On failure: script exits non-zero and prints the failing step + response body.
- Artifacts are written to `${TMPDIR:-/tmp}/my-messaging-smoke` for quick inspection.

## Optional Manual Spot Check

If you need one quick API sanity call:

```bash
curl -sS -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"your_user@test.local","password":"password123"}'
```

## CI/Workflow Suggestion

Use this script as a lightweight integration gate before merge, in addition to unit tests:

```bash
./scripts/smoke-test.sh docker
```
