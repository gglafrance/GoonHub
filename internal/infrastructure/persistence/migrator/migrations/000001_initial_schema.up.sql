CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    username VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    CONSTRAINT uni_users_username UNIQUE (username)
);

CREATE TABLE IF NOT EXISTS videos (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    title VARCHAR(255),
    original_filename VARCHAR(255),
    stored_path VARCHAR(512),
    size BIGINT DEFAULT 0,
    view_count BIGINT DEFAULT 0,
    duration INTEGER DEFAULT 0,
    width INTEGER DEFAULT 0,
    height INTEGER DEFAULT 0,
    thumbnail_path VARCHAR(512),
    sprite_sheet_path VARCHAR(512),
    vtt_path VARCHAR(512),
    sprite_sheet_count INTEGER DEFAULT 0,
    thumbnail_width INTEGER DEFAULT 0,
    thumbnail_height INTEGER DEFAULT 0,
    processing_status VARCHAR(50) DEFAULT 'pending',
    processing_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_videos_deleted_at ON videos (deleted_at);

CREATE TABLE IF NOT EXISTS revoked_tokens (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    token_hash VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    reason VARCHAR(255),
    CONSTRAINT uni_revoked_tokens_token_hash UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_revoked_tokens_expires_at ON revoked_tokens (expires_at);
