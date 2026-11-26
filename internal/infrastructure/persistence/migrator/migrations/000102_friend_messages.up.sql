-- Messages between users (docs/MESSAGES.md): conversations, their participants and the
-- messages, with references to what a message shares (scene, marker, playlist, Goon2gether
-- lobby; resolved at read time, never copied). Only one-to-one ("direct") conversations exist
-- for now; the participants table lets group conversations come later.
CREATE TABLE conversations (
    id BIGSERIAL PRIMARY KEY,
    kind VARCHAR(10) NOT NULL DEFAULT 'direct',
    -- the pair of a direct conversation, smaller id first (one conversation per pair). Set
    -- NULL when the user is deleted: the other side keeps the history ("deleted user")
    direct_low_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    direct_high_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    -- date of the last message (orders the conversation lists)
    last_message_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_conversations_kind CHECK (kind IN ('direct')),
    CONSTRAINT chk_conversations_direct_pair CHECK (direct_low_id IS NULL OR direct_high_id IS NULL OR direct_low_id < direct_high_id)
);

-- One direct conversation per pair (NULLs, after a user deletion, never conflict)
CREATE UNIQUE INDEX idx_conversations_direct_pair ON conversations (direct_low_id, direct_high_id) WHERE kind = 'direct';

CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    -- NULL once the sender's account is deleted ("deleted user")
    sender_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    body TEXT NOT NULL DEFAULT '',
    -- what the message shares (NULL: none). The referenced row going away leaves the kind with
    -- a NULL id: the card shows "not available"
    attachment_kind VARCHAR(16),
    attachment_scene_id BIGINT REFERENCES scenes(id) ON DELETE SET NULL,
    attachment_marker_id BIGINT REFERENCES user_scene_markers(id) ON DELETE SET NULL,
    attachment_playlist_id BIGINT REFERENCES playlists(id) ON DELETE SET NULL,
    -- lobbies are deleted when they close: no foreign key, the card resolves it at read time
    attachment_lobby_uuid UUID,
    -- a scene shared at a given time (seconds)
    attachment_time_s DOUBLE PRECISION,
    edited_at TIMESTAMPTZ,
    -- soft delete: shown as "message deleted", body and attachment are cleared
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_messages_attachment_kind CHECK (attachment_kind IS NULL OR attachment_kind IN ('scene', 'marker', 'playlist', 'lobby')),
    CONSTRAINT chk_messages_attachment_time CHECK (attachment_time_s IS NULL OR attachment_time_s >= 0),
    CONSTRAINT chk_messages_body_length CHECK (char_length(body) <= 10000)
);

-- History pages (newest first by id) and unread counts
CREATE INDEX idx_messages_conversation_id ON messages (conversation_id, id DESC);
CREATE INDEX idx_messages_sender_id ON messages (sender_id);
CREATE INDEX idx_messages_created_at ON messages (created_at);

CREATE TABLE conversation_participants (
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- last message the participant read (unread counts, "seen")
    last_read_message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    -- muted: no header badge count nor toast for this conversation
    muted BOOLEAN NOT NULL DEFAULT false,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX idx_conversation_participants_user_id ON conversation_participants (user_id);

-- Who may message a user, and whether incoming messages show a toast
ALTER TABLE user_social_settings
    ADD COLUMN message_policy VARCHAR(16) NOT NULL DEFAULT 'friends',
    ADD COLUMN message_toasts BOOLEAN NOT NULL DEFAULT true,
    ADD CONSTRAINT chk_user_social_settings_message_policy CHECK (message_policy IN ('everyone', 'mutuals', 'friends', 'nobody'));

-- Share links can start at a time, or play a marker's range (docs/MESSAGES.md, "Share links"):
-- the range and label are copied on the link (the link only exposes them, not the marker)
ALTER TABLE share_links
    ADD COLUMN start_s INTEGER,
    ADD COLUMN end_s INTEGER,
    ADD COLUMN label VARCHAR(100),
    ADD COLUMN marker_id BIGINT REFERENCES user_scene_markers(id) ON DELETE SET NULL,
    ADD CONSTRAINT chk_share_links_start CHECK (start_s IS NULL OR start_s >= 0),
    ADD CONSTRAINT chk_share_links_range CHECK (end_s IS NULL OR (start_s IS NOT NULL AND end_s > start_s));
