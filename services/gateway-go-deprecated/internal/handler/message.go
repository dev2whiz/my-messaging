package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/my-messaging/gateway/internal/broker"
	"github.com/my-messaging/gateway/internal/middleware"
	"github.com/my-messaging/gateway/internal/model"
	"github.com/my-messaging/gateway/internal/store"
)

type MessageHandler struct {
	store  *store.Store
	broker *broker.Broker
}

func NewMessageHandler(s *store.Store, b *broker.Broker) *MessageHandler {
	return &MessageHandler{store: s, broker: b}
}

// POST /messages
// Body: { "recipient_id": "<uuid>", "body": "<text>" }
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	caller := middleware.GetUser(r.Context())

	var req struct {
		RecipientID string `json:"recipient_id"`
		Body        string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.RecipientID == "" || req.Body == "" {
		respondErr(w, http.StatusBadRequest, "recipient_id and body are required")
		return
	}
	if len([]rune(req.Body)) > 4096 {
		respondErr(w, http.StatusUnprocessableEntity, "message body exceeds 4096 characters")
		return
	}
	if req.RecipientID == caller.UserID {
		respondErr(w, http.StatusBadRequest, "cannot send message to yourself")
		return
	}

	// Ensure the recipient exists.
	if _, err := h.store.GetUserByID(r.Context(), req.RecipientID); err != nil {
		respondErr(w, http.StatusNotFound, "recipient not found")
		return
	}

	// Get or create DM conversation.
	conv, err := h.store.GetOrCreateDM(r.Context(), caller.UserID, req.RecipientID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to get conversation")
		return
	}

	msg := &model.Message{
		ID:             uuid.New().String(),
		ConversationID: conv.ID,
		SenderID:       caller.UserID,
		SenderUsername: caller.Username,
		Body:           req.Body,
		SentAt:         Now(),
	}
	if err := h.store.CreateMessage(r.Context(), msg); err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to save message")
		return
	}

	// Publish to RabbitMQ for real-time delivery.
	frame := model.WsFrame{Type: "message", Payload: *msg}
	if err := h.broker.Publish(r.Context(), req.RecipientID, frame); err != nil {
		// Non-fatal: message is persisted; delivery will happen on next poll / reconnect.
		// Stage 2 will add DLX handling here.
	}
	// Also echo back to sender's queue so their own WS sees the sent message.
	_ = h.broker.Publish(r.Context(), caller.UserID, frame)

	respondJSON(w, http.StatusCreated, msg)
}

// GET /conversations/{id}/messages?before=<cursor>&limit=<n>
func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	caller := middleware.GetUser(r.Context())

	// Extract conversation ID from path: /conversations/{id}/messages
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		respondErr(w, http.StatusBadRequest, "invalid path")
		return
	}
	convID := parts[1]

	member, err := h.store.IsMember(r.Context(), convID, caller.UserID)
	if err != nil || !member {
		respondErr(w, http.StatusForbidden, "not a member of this conversation")
		return
	}

	cursor := r.URL.Query().Get("before")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	msgs, err := h.store.ListMessages(r.Context(), convID, cursor, limit)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to load messages")
		return
	}
	respondJSON(w, http.StatusOK, msgs)
}
