package model

import "time"

type User struct {
	ID        string    `db:"id"         json:"id"`
	Username  string    `db:"username"   json:"username"`
	Email     string    `db:"email"      json:"email"`
	Password  string    `db:"password"   json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Conversation struct {
	ID        string    `db:"id"         json:"id"`
	Name      *string   `db:"name"       json:"name"`
	IsGroup   bool      `db:"is_group"   json:"is_group"`
	CreatedBy string    `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Message struct {
	ID             string     `db:"id"              json:"id"`
	ConversationID string     `db:"conversation_id" json:"conversation_id"`
	SenderID       string     `db:"sender_id"       json:"sender_id"`
	SenderUsername string     `db:"sender_username" json:"sender_username,omitempty"`
	Body           string     `db:"body"            json:"body"`
	SentAt         time.Time  `db:"sent_at"         json:"sent_at"`
	ReadAt         *time.Time `db:"read_at"         json:"read_at,omitempty"`
}

// WsFrame is sent over the WebSocket to the client.
type WsFrame struct {
	Type    string  `json:"type"`              // "message" | "ping" | "error"
	Payload Message `json:"payload,omitempty"`
}
