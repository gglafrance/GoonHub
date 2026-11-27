-- Presence (docs/PRESENCE.md): what outlives the in-memory presence state of a user. The
-- online state itself is never stored (live connections, in memory). One row per user who was
-- seen online or chose a presence setting; no row = never seen, visible.
CREATE TABLE user_presence (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- last moment the user was seen online by the others (not updated while invisible)
    last_seen_at TIMESTAMPTZ,
    -- invisible mode: the user appears offline to everyone
    invisible BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
