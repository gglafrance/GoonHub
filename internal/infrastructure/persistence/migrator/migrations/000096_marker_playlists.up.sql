-- Marker playlists: a playlist holds either scenes (playlist_scenes) or markers
-- (playlist_markers), chosen at creation (docs/MARKER_PLAYLISTS.md).
ALTER TABLE playlists
    ADD COLUMN kind VARCHAR(20) NOT NULL DEFAULT 'scenes';

ALTER TABLE playlists
    ADD CONSTRAINT chk_playlists_kind CHECK (kind IN ('scenes', 'markers'));

CREATE INDEX idx_playlists_kind ON playlists (kind);

-- A marker is in a playlist at most once; a marker deleted by its owner leaves every playlist
CREATE TABLE playlist_markers (
    id BIGSERIAL PRIMARY KEY,
    playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    marker_id BIGINT NOT NULL REFERENCES user_scene_markers(id) ON DELETE CASCADE,
    position INT NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_playlist_markers_unique ON playlist_markers (playlist_id, marker_id);
CREATE INDEX idx_playlist_markers_position ON playlist_markers (playlist_id, position);
CREATE INDEX idx_playlist_markers_marker_id ON playlist_markers (marker_id);

-- Resume of a marker playlist: the last marker played (its scene stays in last_scene_id)
ALTER TABLE playlist_progress
    ADD COLUMN last_marker_id BIGINT REFERENCES user_scene_markers(id) ON DELETE SET NULL;
