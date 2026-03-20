# ── Build stage ───────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/main.go

# ── Runtime stage ─────────────────────────────────────────────
FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/gateway .
COPY --from=builder /app/../../migrations ./migrations

EXPOSE 8080
CMD ["./gateway"]
