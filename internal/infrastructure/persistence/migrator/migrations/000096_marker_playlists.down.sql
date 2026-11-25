-- Marker playlists are dropped with their items (there is no scene to fall back on)
ALTER TABLE playlist_progress DROP COLUMN IF EXISTS last_marker_id;

DROP TABLE IF EXISTS playlist_markers;

DELETE FROM playlists WHERE kind = 'markers';

DROP INDEX IF EXISTS idx_playlists_kind;
ALTER TABLE playlists DROP CONSTRAINT IF EXISTS chk_playlists_kind;
ALTER TABLE playlists DROP COLUMN IF EXISTS kind;
