package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/my-messaging/gateway/internal/model"
)

type Store struct {
	db *sqlx.DB
}

func New(dsn string) (*Store, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &Store{db: db}, nil
}

func (s *Store) DB() *sqlx.DB { return s.db }

// ── Users ─────────────────────────────────────────────────────────────────────

func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, email, password) VALUES ($1,$2,$3,$4)`,
		u.ID, u.Username, u.Email, u.Password)
	return err
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := s.db.GetContext(ctx, u, `SELECT * FROM users WHERE email=$1`, email)
	return u, err
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := s.db.GetContext(ctx, u, `SELECT * FROM users WHERE id=$1`, id)
	return u, err
}

func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := s.db.SelectContext(ctx, &users,
		`SELECT id, username, email, created_at, updated_at FROM users ORDER BY username`)
	return users, err
}

// ── Conversations ─────────────────────────────────────────────────────────────

// GetOrCreateDM returns an existing DM conversation between two users, or creates one.
func (s *Store) GetOrCreateDM(ctx context.Context, userA, userB string) (*model.Conversation, error) {
	conv := &model.Conversation{}
	err := s.db.GetContext(ctx, conv, `
		SELECT c.* FROM conversations c
		JOIN conversation_members m1 ON m1.conversation_id = c.id AND m1.user_id = $1
		JOIN conversation_members m2 ON m2.conversation_id = c.id AND m2.user_id = $2
		WHERE c.is_group = FALSE
		LIMIT 1`, userA, userB)
	if err == nil {
		return conv, nil
	}

	// create new DM
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := tx.QueryRowxContext(ctx,
		`INSERT INTO conversations (created_by) VALUES ($1) RETURNING *`, userA,
	).StructScan(conv); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1,$2),($1,$3)`,
		conv.ID, userA, userB); err != nil {
		return nil, err
	}
	return conv, tx.Commit()
}

func (s *Store) IsMember(ctx context.Context, convID, userID string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`,
		convID, userID)
	return count > 0, err
}

// ListUserConversations returns all conversations the user belongs to.
func (s *Store) ListUserConversations(ctx context.Context, userID string) ([]model.Conversation, error) {
	var convs []model.Conversation
	err := s.db.SelectContext(ctx, &convs, `
		SELECT c.* FROM conversations c
		JOIN conversation_members m ON m.conversation_id = c.id
		WHERE m.user_id = $1
		ORDER BY c.created_at DESC`, userID)
	return convs, err
}

// ── Messages ──────────────────────────────────────────────────────────────────

func (s *Store) CreateMessage(ctx context.Context, m *model.Message) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (id, conversation_id, sender_id, body) VALUES ($1,$2,$3,$4)`,
		m.ID, m.ConversationID, m.SenderID, m.Body)
	return err
}

// ListMessages returns up to `limit` messages before the cursor (exclusive).
// cursor="" means return the latest.
func (s *Store) ListMessages(ctx context.Context, convID, cursor string, limit int) ([]model.Message, error) {
	var rows []model.Message
	if cursor == "" {
		err := s.db.SelectContext(ctx, &rows, `
			SELECT m.*, u.username AS sender_username
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1
			ORDER BY m.sent_at DESC
			LIMIT $2`, convID, limit)
		return rows, err
	}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT m.*, u.username AS sender_username
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.conversation_id = $1 AND m.sent_at < (SELECT sent_at FROM messages WHERE id = $2)
		ORDER BY m.sent_at DESC
		LIMIT $3`, convID, cursor, limit)
	return rows, err
}
