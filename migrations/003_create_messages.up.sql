CREATE TABLE messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    body            TEXT NOT NULL CHECK (char_length(body) <= 4096),
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at         TIMESTAMPTZ                         -- Stage 2: read receipt
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id, sent_at DESC);
