# ── Build stage ───────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY services/gateway/go.mod services/gateway/go.sum ./
RUN go mod download

COPY services/gateway/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/main.go

# ── Runtime stage ─────────────────────────────────────────────
FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/gateway .
COPY migrations ./migrations

EXPOSE 8080
CMD ["./gateway"]
