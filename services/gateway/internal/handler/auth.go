package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/my-messaging/gateway/internal/auth"
	"github.com/my-messaging/gateway/internal/broker"
	"github.com/my-messaging/gateway/internal/middleware"
	"github.com/my-messaging/gateway/internal/model"
	"github.com/my-messaging/gateway/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	store   *store.Store
	authSvc *auth.Service
	broker  *broker.Broker
}

func NewAuthHandler(s *store.Store, a *auth.Service, b *broker.Broker) *AuthHandler {
	return &AuthHandler{store: s, authSvc: a, broker: b}
}

// POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Username == "" || req.Email == "" || len(req.Password) < 8 {
		respondErr(w, http.StatusBadRequest, "username, email and password (min 8 chars) required")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &model.User{
		ID:       uuid.New().String(),
		Username: req.Username,
		Email:    req.Email,
		Password: string(hash),
	}
	if err := h.store.CreateUser(r.Context(), user); err != nil {
		if strings.Contains(err.Error(), "unique") {
			respondErr(w, http.StatusConflict, "username or email already taken")
			return
		}
		respondErr(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Declare this user's RabbitMQ queue so messages can be routed immediately.
	if err := h.broker.DeclareUserQueue(user.ID); err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to setup message queue")
		return
	}

	token, err := h.authSvc.IssueToken(user.ID, user.Username)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"token": token,
		"user":  user,
	})
}

// POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), strings.ToLower(req.Email))
	if err != nil {
		respondErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		respondErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.authSvc.IssueToken(user.ID, user.Username)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

// POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tokenStr == "" {
		respondErr(w, http.StatusBadRequest, "missing token")
		return
	}
	claims, err := h.authSvc.ValidateToken(r.Context(), tokenStr)
	if err != nil {
		respondErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	if err := h.authSvc.RevokeToken(r.Context(), tokenStr, claims.ExpiresAt.Time); err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// GET /auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	user, err := h.store.GetUserByID(r.Context(), u.UserID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "user not found")
		return
	}
	respondJSON(w, http.StatusOK, user)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respondErr(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// Expose for other handlers
var RespondJSON = respondJSON
var RespondErr = respondErr
var Now = func() time.Time { return time.Now().UTC() }
