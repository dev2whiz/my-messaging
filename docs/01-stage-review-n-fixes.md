## Plan: Stage 1 Review Remediation Backlog

Backlog is prepared and prioritized for immediate execution with Critical and High issues first, while preserving current UI behavior.

**Last updated: 21 March 2026** — all P0 and P1 items resolved. P2 items pending.

**Steps**
1. Execute Stage 1 Hotfix items P0-1 through P0-5 first to restore end-to-end correctness and remove immediate security leakage.
2. Execute Security Stabilization items P1-1 through P1-5 after Stage 1 flow is green.
3. Execute Hardening and Documentation items P2-1 through P2-3 in parallel where possible.
4. Gate each item with acceptance criteria and tests before moving to the next dependency block.

**Backlog (Ready To Track)**

1. ID: P0-1
Status: ✅ FIXED
Priority: Critical
Title: Login contract mismatch (email vs username)
Risk: Users cannot authenticate from current UI
Scope: Backend request contract and repository lookup
Implementation tasks:
- Change login request handling in AuthController to align with frontend email-based login in LoginPage and client.
- Use UserRepository.findByEmail in login flow.
Acceptance criteria: Email login succeeds and returns token/user payload.
Tests: API contract test for POST login using email.
Dependencies: None
Target: Stage 1 hotfix
Resolution: AuthController.login() uses LoginRequest(email, password) and UserRepository.findByEmail(). Frontend LoginPage and api/client.ts both send email. Fully aligned.

2. ID: P0-2
Status: ✅ FIXED
Priority: Critical
Title: Spring model to SQL schema mismatch
Risk: Persistence and reads fail or return malformed entities
Scope: JDBC records vs migration columns
Implementation tasks:
- Align fields/column mappings across User model, Message model, Conversation model with users, conversations, and messages migrations.
Acceptance criteria: Register, send message, fetch history all persist and hydrate correctly.
Tests: Integration tests for insert/select across all three tables.
Dependencies: P0-1 recommended first
Target: Stage 1 hotfix
Resolution: All @Column mappings verified against migration DDL. Added migration 004 to drop the NOT NULL constraint on conversations.created_by that contradicted ON DELETE SET NULL. Added @JsonProperty("isGroup") to Conversation to prevent Jackson serializing the field as "group" via the isXxx() accessor heuristic.

3. ID: P0-3
Status: ✅ FIXED
Priority: Critical
Title: Password leakage in API responses
Risk: Credential material exposure
Scope: DTO boundary in auth/user endpoints
Implementation tasks:
- Stop returning raw entity objects from AuthController and UserController.
- Introduce response DTOs excluding password fields from User model.
Acceptance criteria: No password or hash appears in any API response.
Tests: Serialization tests and endpoint response assertions.
Dependencies: P0-2
Target: Stage 1 hotfix
Resolution: UserDto record (id, username, email, created_at, updated_at) is the only type returned by AuthController and UserController. No password field is present anywhere in the response surface.

4. ID: P0-4
Status: ✅ FIXED
Priority: Critical
Title: Message payload shape mismatch with UI expectations
Risk: Chat rendering and thread state break
Scope: API response fields for message list/ws payload
Implementation tasks:
- Align backend payload fields to what frontend types and ChatPage consume, including sender username and timestamp naming.
- Update repository/controller projection logic in MessageController.
Acceptance criteria: Message list and live frames render without client-side field fallbacks.
Tests: Contract tests for messages endpoint and WebSocket frame payload shape.
Dependencies: P0-2
Target: Stage 1 hotfix
Resolution: MessageWithSender record exposes exactly conversation_id, sender_id, sender_username, body, sent_at, read_at matching the frontend Message type. Jackson2JsonMessageConverter now configured with JavaTimeModule + WRITE_DATES_AS_TIMESTAMPS=false in both RabbitConfig and WebSocketConfig so WS frames carry ISO-8601 timestamps (fixes "56 years ago" rendering bug caused by epoch-seconds being treated as milliseconds).

5. ID: P0-5
Status: ✅ FIXED
Priority: Critical
Title: Current-user route mismatch
Risk: Broken me/profile bootstrap flow
Scope: Route parity between frontend and backend
Implementation tasks:
- Align frontend me call with backend implementation in UserController, either by route alias or client update.
Acceptance criteria: Me endpoint resolves successfully during app startup/auth refresh.
Tests: API route regression test.
Dependencies: P0-3
Target: Stage 1 hotfix
Resolution: /auth/me is implemented in AuthController and /users/me in UserController. Frontend api/client.ts calls /auth/me. Both routes return UserDto. Fully aligned.

6. ID: P1-1
Status: ✅ FIXED
Priority: High
Title: Overly permissive CORS and WS origins
Risk: Expanded cross-origin attack surface
Scope: HTTP CORS and WebSocket origin policy
Implementation tasks:
- Replace wildcard policy in SecurityConfig with explicit allowlist from env/config.
- Restrict WS origins in WebSocketConfig.
Acceptance criteria: Only approved origins can call APIs and open WS.
Tests: Origin-based integration tests for allowed/blocked cases.
Dependencies: P0 set complete
Target: Stage 1.5 or Stage 2 early
Resolution: app.allowed-origins property added to application.yml, overridable via ALLOWED_ORIGINS env var (defaults to http://localhost:5173,http://localhost:4173). SecurityConfig reads the value via @Value and uses setAllowedOrigins() (not wildcard pattern). WebSocketConfig uses the same value for setAllowedOrigins(). SecurityConfigTest updated to assert explicit origin list.

7. ID: P1-2
Status: ✅ FIXED
Priority: High
Title: Unsafe token transport and handling
Risk: Token leakage via query strings/logs/storage
Scope: JWT extraction, WS connection strategy, frontend token storage posture
Implementation tasks:
- Tighten token extraction in JwtAuthenticationFilter to avoid broad query token acceptance.
- Rework WS token usage currently in useWebSocket and parser logic in MessagingWebSocketHandler.
- Review local storage persistence path in auth store and api client.
Acceptance criteria: No token in non-WS query paths; minimized token exposure in logs/browser artifacts.
Tests: Security regression tests for rejected query-token HTTP calls and WS auth path.
Dependencies: P1-1
Target: Stage 2 hardening
Resolution: JwtAuthenticationFilter.getJwtFromRequest() now only accepts ?token= query parameter when requestURI == /ws. All other HTTP paths require Authorization: Bearer header. JwtAuthenticationFilterTest updated to set /ws URI on the query-param test. LocalStorage token storage is a known remaining risk (acceptable for Stage 1; cookie-based auth is a Stage 3 concern).

8. ID: P1-3
Status: ✅ FIXED
Priority: High
Title: JWT revocation key strategy
Risk: Inefficient/fragile blocklist keys
Scope: Revocation storage design
Implementation tasks:
- Improve keying in JwtTokenProvider to claim-based revocation identifiers with TTL parity.
Acceptance criteria: Revoked tokens are reliably rejected with bounded key size growth.
Tests: Unit tests for token generation/revocation/expiry behavior.
Dependencies: P1-2
Target: Stage 2 hardening
Resolution: generateToken() now sets .id(UUID.randomUUID().toString()) (jti claim). validateToken() and invalidateToken() use jwt:blocklist:<jti> as the Redis key — a fixed 36-char UUID — instead of the full compact token string (hundreds of bytes). TTL parity with token expiry is preserved. JwtTokenProviderTest updated to match on key prefix rather than full token string.

9. ID: P1-4
Status: ✅ FIXED
Priority: High
Title: Direct conversation query ambiguity
Risk: Wrong thread resolution for DMs
Scope: Repository query semantics
Implementation tasks:
- Tighten conversation lookup in ConversationRepository to guarantee direct conversation constraints.
Acceptance criteria: DM lookup never resolves to group/shared threads.
Tests: Repository integration tests with DM + group fixtures.
Dependencies: P0-2
Target: Stage 2 hardening
Resolution: findDirectConversationBetween() query now includes AND c.is_group = FALSE, preventing it from ever returning a group conversation that happens to contain both participants.

10. ID: P1-5
Status: ✅ FIXED
Priority: High
Title: WS message delivery reliability under failures
Risk: Message loss on disconnect/send error
Scope: Rabbit consumer ack strategy
Implementation tasks:
- Replace auto-ack behavior in MessagingWebSocketHandler with explicit ack/nack policy tied to WS send success path.
Acceptance criteria: Messages are not dropped when client disconnects mid-delivery.
Tests: Integration test with forced disconnect during delivery.
Dependencies: P1-4
Target: Stage 2 hardening
Resolution: MessagingWebSocketHandler uses AcknowledgeMode.MANUAL with setDefaultRequeueRejected(true). On successful WS send: channel.basicAck(deliveryTag, false). On closed session or exception: channel.basicNack(deliveryTag, false, true). Queue and binding are idempotently declared at both login and WS connect time.

11. ID: P2-1
Status: ⏳ PENDING
Priority: Medium
Title: Global JSON naming consistency
Risk: Ongoing API contract drift and ad hoc mapping
Scope: Serialization strategy across REST and WS
Implementation tasks:
- Standardize naming policy so backend output matches frontend expectations in types.
Acceptance criteria: Uniform field naming across all endpoints/frames.
Tests: Snapshot contract tests for representative payloads.
Dependencies: P0-4
Target: Stage 2+

12. ID: P2-2
Status: ⏳ PENDING
Priority: Medium
Title: Dependency security posture and lifecycle updates
Risk: Known vulnerable dependencies and support window risk
Scope: Maven dependency updates and policy
Implementation tasks:
- Address warnings identified in pom and verify framework support timeline.
Acceptance criteria: Vulnerability scan clean for accepted severity threshold.
Tests: Dependency scan in CI.
Dependencies: None
Target: Stage 2+

13. ID: P2-3
Status: ⏳ PENDING
Priority: Medium
Title: Documentation drift (Go narrative vs Spring runtime)
Risk: Invalid verification and onboarding confusion
Scope: Stage docs and README correction
Implementation tasks:
- Align README with Spring implementation.
- Reconcile stage claims in walkthrough doc.
- Keep migration intent consistent with Spring migration plan.
Acceptance criteria: Docs accurately describe current architecture and commands.
Tests: Manual doc verification with fresh setup run.
Dependencies: P0 stabilization complete
Target: Stage 1 hotfix tail

**Milestone Split**
1. Milestone A (Immediate): P0-1 to P0-5, then P1-1 and P2-3. — ✅ P0-1 to P0-5 and P1-1 complete. P2-3 pending.
2. Milestone B (Hardening): P1-2 to P1-5, then P2-1 and P2-2. — ✅ P1-2 to P1-5 complete. P2-1 and P2-2 pending.

**Verification**
1. API contract checks for auth, me, users, messages, history.
2. End-to-end test: two users register, login, send and receive DM in real time.
3. Security checks: no password/hash responses, restricted origins, token handling constraints.
4. Reliability checks: reconnect and delivery behavior under WS interruption.

**Test results as of 21 March 2026**
- Spring Boot: 50 tests, 0 failures, 0 errors — BUILD SUCCESS
- Frontend: npm run build — ✅ 355 modules, no TypeScript errors
