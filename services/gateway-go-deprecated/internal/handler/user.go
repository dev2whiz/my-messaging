package handler

import (
	"encoding/json"
	"net/http"

	"github.com/my-messaging/gateway/internal/store"
)

type UserHandler struct {
	store *store.Store
}

func NewUserHandler(s *store.Store) *UserHandler {
	return &UserHandler{store: s}
}

// GET /users  — list all registered users (excluding the caller)
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	respondJSON(w, http.StatusOK, users)
}

// GET /conversations — list conversations the current user belongs to
func (h *UserHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(nil) // suppress unused import if needed
	respondErr(w, http.StatusNotImplemented, "use /messages to start a DM")
}
