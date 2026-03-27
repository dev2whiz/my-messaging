# syntax=docker/dockerfile:1.7

# ── Build stage ────────────────────────────────────────────────
# pom.xml and src are copied separately so the dependency download step
# is only re-run when pom.xml changes.  The BuildKit cache mount keeps
# ~/.m2 across builds; no separate deps stage is needed.
FROM maven:3.9.6-eclipse-temurin-21-alpine AS builder

WORKDIR /app
COPY pom.xml .
RUN --mount=type=cache,target=/root/.m2 \
	mvn -B -ntp dependency:go-offline

COPY src ./src
RUN --mount=type=cache,target=/root/.m2 \
	mvn -B -ntp clean package -DskipTests

# ── Runtime stage ──────────────────────────────────────────────
FROM eclipse-temurin:21-jre-alpine

WORKDIR /app
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder --chown=app:app /app/target/gateway-*.jar app.jar

USER app
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "app.jar"]
