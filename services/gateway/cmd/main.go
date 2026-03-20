package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redis/go-redis/v9"

	"github.com/my-messaging/gateway/internal/auth"
	"github.com/my-messaging/gateway/internal/broker"
	"github.com/my-messaging/gateway/internal/handler"
	"github.com/my-messaging/gateway/internal/middleware"
	"github.com/my-messaging/gateway/internal/store"
)

func main() {
	// ── Config from environment ───────────────────────────────────────────────
	pgHost := getEnv("POSTGRES_HOST", "localhost")
	pgPort := getEnv("POSTGRES_PORT", "5432")
	pgUser := getEnv("POSTGRES_USER", "msguser")
	pgPass := getEnv("POSTGRES_PASSWORD", "msgpassword")
	pgDB := getEnv("POSTGRES_DB", "messaging")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", pgUser, pgPass, pgHost, pgPort, pgDB)

	valkeyAddr := getEnv("VALKEY_ADDR", "localhost:6379")
	valkeyPass := getEnv("VALKEY_PASSWORD", "")

	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	port := getEnv("GATEWAY_PORT", "8080")
	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production-use-32-chars")
	jwtExpiryHrs, _ := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	corsOrigin := getEnv("CORS_ORIGIN", "http://localhost:5173")

	// ── Postgres + migrations ─────────────────────────────────────────────────
	st, err := store.New(dsn)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	m, err := migrate.New("file:///app/migrations", dsn)
	if err != nil {
		log.Fatalf("migrate: new: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate: up: %v", err)
	}
	log.Println("migrations: applied")

	// ── Valkey ────────────────────────────────────────────────────────────────
	vk := redis.NewClient(&redis.Options{
		Addr:     valkeyAddr,
		Password: valkeyPass,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := vk.Ping(ctx).Err(); err != nil {
		log.Fatalf("valkey: ping: %v", err)
	}
	log.Println("valkey: connected")

	// ── RabbitMQ ──────────────────────────────────────────────────────────────
	brk, err := broker.New(rabbitURL)
	if err != nil {
		log.Fatalf("broker: %v", err)
	}
	defer brk.Close()
	log.Println("rabbitmq: connected")

	// ── Services ──────────────────────────────────────────────────────────────
	authSvc := auth.NewService(jwtSecret, jwtExpiryHrs, vk)

	// ── Handlers ──────────────────────────────────────────────────────────────
	authH := handler.NewAuthHandler(st, authSvc, brk)
	userH := handler.NewUserHandler(st)
	msgH := handler.NewMessageHandler(st, brk)
	wsH := handler.NewWsHandler(brk)

	// ── Router ────────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("POST /auth/register", authH.Register)
	mux.HandleFunc("POST /auth/login", authH.Login)

	// Protected — wrap with auth middleware inline
	authMW := middleware.Auth(authSvc)

	mux.Handle("POST /auth/logout", authMW(http.HandlerFunc(authH.Logout)))
	mux.Handle("GET /auth/me", authMW(http.HandlerFunc(authH.Me)))

	mux.Handle("GET /users", authMW(http.HandlerFunc(userH.List)))

	mux.Handle("POST /messages", authMW(http.HandlerFunc(msgH.Send)))
	mux.Handle("GET /conversations/", authMW(http.HandlerFunc(msgH.List)))

	// WebSocket — auth via ?token= query param
	wsMW := middleware.TokenFromQuery(authSvc)
	mux.Handle("/ws", wsMW(wsH))

	// ── Server ────────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      middleware.CORS(corsOrigin)(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("gateway: listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("gateway: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
