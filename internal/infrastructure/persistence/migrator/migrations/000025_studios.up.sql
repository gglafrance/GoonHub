CREATE TABLE IF NOT EXISTS studios (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    name VARCHAR(255) NOT NULL,
    short_name VARCHAR(100),
    url VARCHAR(512),
    description TEXT,
    rating DECIMAL(3,1),

    logo VARCHAR(512),
    favicon VARCHAR(512),
    poster VARCHAR(512),

    porndb_id VARCHAR(100),
    parent_id BIGINT REFERENCES studios(id),
    network_id BIGINT REFERENCES studios(id)
);

ALTER TABLE videos ADD COLUMN studio_id BIGINT REFERENCES studios(id) ON DELETE SET NULL;

CREATE INDEX idx_studios_uuid ON studios(uuid);
CREATE INDEX idx_studios_name ON studios(name);
CREATE INDEX idx_studios_deleted_at ON studios(deleted_at);
CREATE INDEX idx_studios_porndb_id ON studios(porndb_id);
CREATE INDEX idx_videos_studio_id ON videos(studio_id);

-- Migrate existing studio strings to studio records
INSERT INTO studios (name)
SELECT DISTINCT studio FROM videos WHERE studio != '' AND studio IS NOT NULL;

-- Link videos to their studio records
UPDATE videos v SET studio_id = s.id FROM studios s WHERE v.studio = s.name AND v.studio != '';
