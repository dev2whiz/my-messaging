# Messaging Platform — Spring Boot Migration Plan

This plan supersedes the previous Go implementation to adopt **Spring Boot 3.x (Java 21+)** as the primary API Gateway, achieving full feature parity while retaining the Go service as a deprecated reference.

## User Review Required
> [!WARNING]
> This plan involves replacing the core API in the `docker-compose.yml` from Go to Spring Boot. The existing Go code in `services/gateway` will be renamed to `services/gateway-go-deprecated`. I plan to use the `golang-migrate/migrate` Docker image as a standalone service in `docker-compose.yml` to apply our existing SQL migrations automatically, thus keeping the existing SQL files exactly as they are without needing to convert them to Flyway Java migrations. Are you comfortable with this approach?

## Architecture Adjustments

| Component | Choice | Rationale |
|---|---|---|
| Language & Runtime | **Java 21+ Virtual Threads** | Match Go's goroutine concurrency scale for WebSockets natively without WebFlux complexity. `spring.threads.virtual.enabled=true`.|
| Web Framework | **Spring Boot 3.x (WebMVC)** | Standard, robust, and now scales blockingly using Virtual Threads. |
| RabbitMQ | **Spring AMQP** | Uses `RabbitTemplate` and `@RabbitListener` for zero-boilerplate pub/sub and consumer definition. |
| Database Ops | **Spring Data JDBC** | Clean, fast data mapping, avoiding JPA/Hibernate lazy-loading issues and matching the explicit SQL nature of the former Go `sqlx` setup. |
| Cache / Presence | **Spring Data Redis** | Perfect compatibility with our Valkey 8.x broker for JWT blocklists. |
| Dependency Management| **Maven** | Industry standard `pom.xml`. |

## Proposed Changes

---
### Infrastructure & Deprecation

#### [MODIFY] [docker-compose.yml](file:///Users/deva/Workspace/ai-projects/my-messaging/docker-compose.yml)
- Update the `gateway` service build context to `./services/gateway-spring`.
- Expose Spring Boot on port `8080`.
- Add a dedicated `migrate` container using the `migrate/migrate` alpine image to automatically apply the existing `migrations/*.sql` on startup before Spring Boot boots.

#### [RENAME] Go Gateway
- Rename `services/gateway` to `services/gateway-go-deprecated` to cleanly preserve it.

---
### Spring Boot App (New)
The new Spring Boot application will reside in `services/gateway-spring`.

#### [NEW] Configuration
- `pom.xml`: Add dependencies (`spring-boot-starter-web`, `websocket`, `amqp`, `data-jdbc`, `data-redis`, `security`, `jjwt`, `postgresql`).
- `application.yml`: Configure PostgreSQL url, Valkey host, RabbitMQ host, and enable Virtual Threads.

#### [NEW] Domain Models
- Define Java 21 `Record` types for `User`, `Conversation`, `Message`, and `WsFrame` to perfectly mirror the frontend TypeScript models.

#### [NEW] Security Layer (JWT & Valkey)
- Implement `JwtTokenProvider` to generate and parse HS256 tokens using `jjwt`.
- Implement a `JwtAuthenticationFilter` attached to Spring Security to intercept requests, validate the token, and check if it exists in the Valkey blocklist via `StringRedisTemplate`.

#### [NEW] HTTP REST Controllers
- `/auth/register`: Hash passwords (BCrypt via Spring Security), insert into JDBC, returning JWT.
- `/auth/login`: Verify credentials, returning JWT.
- `/auth/logout`: Add current JWT ID (`jti`) to Valkey blocklist.
- `/users`: Fetch all active users via JDBC.
- `/messages`: Validate limit (≤ 4096), save to DB via JDBC, publish to AMQP via `RabbitTemplate`.
- `/conversations/{id}/messages`: Fetch chunked, cursor-paginated message history.

#### [NEW] RabbitMQ AMQP / WebSocket Bridge
- **Config**: Auto-declare the `messaging` topic exchange.
- **Consumer**: Rather than writing manual channel loops, use RabbitAdmin to programmably declare a queue `user.{id}` and bind it cleanly when a user registers, pulling messages via `@RabbitListener` or programmatic `SimpleMessageListenerContainer`.
- **WebSocketHandler**: Implement `TextWebSocketHandler`. Authenticate connections via the `?token=` query param. Map active WebSocket `Session`s in memory. When a message is consumed from RabbitMQ, push it down the corresponding `WebSocketSession`.

## Verification Plan

### Automated Tests
- Stand up the full stack via `docker compose up --build -d`.
- Validate the Spring Boot gateway boots successfully, connects to the infra, and the `migrate` container applies the SQL.

### Manual Verification
- Use the existing front-end React app (`localhost:5173`) to register a new user, login, and send messages.
- Verify that messages bridge seamlessly from the Spring Boot REST endpoint -> Postgres -> RabbitMQ -> Spring WebSockets -> Browser.

## Delivery Workflow By Stage

### Stage 1-4 Command Strategy
- Use a **root Makefile** as the primary developer command surface.
- Root targets delegate to service/module-specific Makefiles in `services/gateway-spring/` and `web/`.
- Keep executable logic in shell scripts under `scripts/` (for example `scripts/smoke-test.sh`), with Make acting only as the orchestration entrypoint.
- Supported developer flows should cover:
	- local infrastructure bootstrap (`make infra-up`, `make migrate`)
	- local service run (`make gateway-run`, `make web-run`)
	- full container stack (`make up`, `make down`)
	- build/test (`make build`, `make test`)
	- integration sanity run (`make smoke`, `make smoke-local`, `make check`)

### Stage 5 Production Ready Additions
- Add a **CI workflow** in Stage 5, not earlier.
- CI should become the enforceable quality gate and run at minimum:
	- backend unit tests
	- frontend build / type check
	- docker-based smoke integration test
- Publish immutable build artifacts / container images from CI.
- Extend Stage 5 with deployment workflow concerns: image scanning, release tagging, rollout validation, and environment promotion.
