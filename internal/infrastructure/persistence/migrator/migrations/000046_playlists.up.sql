-- Playlists table
CREATE TABLE playlists (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    visibility VARCHAR(20) NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_playlists_uuid ON playlists (uuid);
CREATE INDEX idx_playlists_user_id ON playlists (user_id);
CREATE INDEX idx_playlists_visibility ON playlists (visibility) WHERE visibility = 'public';

-- Playlist scenes junction table
CREATE TABLE playlist_scenes (
    id BIGSERIAL PRIMARY KEY,
    playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    position INT NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_playlist_scenes_playlist_id ON playlist_scenes (playlist_id);
CREATE UNIQUE INDEX idx_playlist_scenes_unique ON playlist_scenes (playlist_id, scene_id);
CREATE INDEX idx_playlist_scenes_position ON playlist_scenes (playlist_id, position);

-- Playlist tags junction table (reuses existing tags table)
CREATE TABLE playlist_tags (
    playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_playlist_tags_unique ON playlist_tags (playlist_id, tag_id);

-- Playlist likes table
CREATE TABLE playlist_likes (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_playlist_likes_unique ON playlist_likes (user_id, playlist_id);

-- Playlist progress table (resume support)
CREATE TABLE playlist_progress (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    last_scene_id BIGINT REFERENCES scenes(id) ON DELETE SET NULL,
    last_position_s DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_playlist_progress_unique ON playlist_progress (user_id, playlist_id);

-- Insert playlist permissions
INSERT INTO permissions (name, description, created_at)
VALUES
    ('playlists:create', 'Create new playlists', NOW()),
    ('playlists:delete', 'Delete own playlists', NOW()),
    ('playlists:edit', 'Edit own playlists', NOW()),
    ('playlists:view_public', 'View public playlists from other users', NOW());

-- Assign playlist permissions to admin, moderator, and user roles
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name IN ('admin', 'moderator', 'user')
  AND p.name IN ('playlists:create', 'playlists:delete', 'playlists:edit', 'playlists:view_public');
