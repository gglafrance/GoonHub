CREATE TABLE pool_config (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    metadata_workers INTEGER NOT NULL DEFAULT 3,
    thumbnail_workers INTEGER NOT NULL DEFAULT 1,
    sprites_workers INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
