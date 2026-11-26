-- Goon2gether: synchronized watch lobbies (docs/GOON2GETHER.md) and server custom emotes.

CREATE TABLE lobbies (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL,
    invite_code VARCHAR(16) NOT NULL,
    name VARCHAR(100) NOT NULL,
    visibility VARCHAR(10) NOT NULL DEFAULT 'private',
    host_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    control_open BOOLEAN NOT NULL DEFAULT FALSE,
    queue_open BOOLEAN NOT NULL DEFAULT FALSE,
    wait_for_all BOOLEAN NOT NULL DEFAULT FALSE,
    max_members INTEGER NOT NULL DEFAULT 0,
    current_item_id BIGINT,
    position_s DOUBLE PRECISION NOT NULL DEFAULT 0,
    playback_rate DOUBLE PRECISION NOT NULL DEFAULT 1,
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_lobbies_visibility CHECK (visibility IN ('public', 'private')),
    CONSTRAINT chk_lobbies_max_members CHECK (max_members >= 0),
    CONSTRAINT chk_lobbies_playback_rate CHECK (playback_rate > 0)
);

CREATE UNIQUE INDEX idx_lobbies_uuid ON lobbies (uuid);
CREATE UNIQUE INDEX idx_lobbies_invite_code ON lobbies (invite_code);
CREATE INDEX idx_lobbies_public_active ON lobbies (last_active_at DESC) WHERE visibility = 'public';
CREATE INDEX idx_lobbies_host_user_id ON lobbies (host_user_id);
CREATE INDEX idx_lobbies_last_active_at ON lobbies (last_active_at);

CREATE TABLE lobby_members (
    lobby_id BIGINT NOT NULL REFERENCES lobbies(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_control BOOLEAN NOT NULL DEFAULT FALSE,
    banned_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lobby_id, user_id)
);

CREATE INDEX idx_lobby_members_user_id ON lobby_members (user_id);

CREATE TABLE lobby_queue_items (
    id BIGSERIAL PRIMARY KEY,
    lobby_id BIGINT NOT NULL REFERENCES lobbies(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    status VARCHAR(10) NOT NULL DEFAULT 'queued',
    position INTEGER NOT NULL DEFAULT 0,
    start_s DOUBLE PRECISION,
    end_s DOUBLE PRECISION,
    label VARCHAR(100),
    added_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_lobby_queue_items_status CHECK (status IN ('queued', 'suggested')),
    CONSTRAINT chk_lobby_queue_items_start CHECK (start_s IS NULL OR start_s >= 0),
    CONSTRAINT chk_lobby_queue_items_range CHECK (end_s IS NULL OR end_s > COALESCE(start_s, 0))
);

CREATE INDEX idx_lobby_queue_items_lobby_status_position ON lobby_queue_items (lobby_id, status, position);
CREATE INDEX idx_lobby_queue_items_scene_id ON lobby_queue_items (scene_id);

-- The item playing: cleared when the item is deleted (scene deleted, item removed)
ALTER TABLE lobbies
    ADD CONSTRAINT fk_lobbies_current_item FOREIGN KEY (current_item_id)
    REFERENCES lobby_queue_items(id) ON DELETE SET NULL;

CREATE TABLE emotes (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(32) NOT NULL,
    filename VARCHAR(64) NOT NULL,
    content_type VARCHAR(32) NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    size_bytes INTEGER NOT NULL,
    animated BOOLEAN NOT NULL DEFAULT FALSE,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_emotes_name CHECK (name ~ '^[A-Za-z0-9_]{2,32}$'),
    CONSTRAINT chk_emotes_content_type CHECK (content_type IN ('image/png', 'image/gif', 'image/webp'))
);

CREATE UNIQUE INDEX idx_emotes_name_lower ON emotes (LOWER(name));
CREATE UNIQUE INDEX idx_emotes_filename ON emotes (filename);
