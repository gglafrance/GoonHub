-- Emoji and emote reactions on the messages between users (docs/MESSAGES.md, "Reactions"). One
-- row per message, user and reaction key: a Unicode emoji, or ":name:" of a server custom emote
-- (kept as text: a deleted emote then shows as its :name:). Deleting a message (soft delete)
-- removes its reactions in the application; a hard delete (retention) or a user deletion
-- removes them with the cascade.
CREATE TABLE message_reactions (
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id, emoji),
    CONSTRAINT chk_message_reactions_emoji CHECK (char_length(emoji) BETWEEN 1 AND 64)
);

-- A user's reactions (account deletion cascade)
CREATE INDEX idx_message_reactions_user_id ON message_reactions (user_id);
