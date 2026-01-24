CREATE TABLE IF NOT EXISTS tags (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    color VARCHAR(7) NOT NULL DEFAULT '#6B7280',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS video_tags (
    id SERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    UNIQUE(video_id, tag_id)
);

CREATE INDEX idx_video_tags_video_id ON video_tags(video_id);
CREATE INDEX idx_video_tags_tag_id ON video_tags(tag_id);

-- Seed premade tags
INSERT INTO tags (name, color) VALUES
    ('Anal', '#6B7280'),
    ('Blowjob', '#6B7280'),
    ('Cumshot', '#6B7280'),
    ('Deepthroat', '#6B7280'),
    ('Double Penetration', '#6B7280'),
    ('Fingering', '#6B7280'),
    ('Handjob', '#6B7280'),
    ('Gangbang', '#6B7280'),
    ('Kissing', '#6B7280'),
    ('Masturbation', '#6B7280')
ON CONFLICT (name) DO NOTHING;
