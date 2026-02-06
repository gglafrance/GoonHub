-- Remove role_permissions for playlist permissions
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('playlists:create', 'playlists:delete', 'playlists:edit', 'playlists:view_public')
);

-- Remove playlist permissions
DELETE FROM permissions WHERE name IN ('playlists:create', 'playlists:delete', 'playlists:edit', 'playlists:view_public');

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS playlist_progress;
DROP TABLE IF EXISTS playlist_likes;
DROP TABLE IF EXISTS playlist_tags;
DROP TABLE IF EXISTS playlist_scenes;
DROP TABLE IF EXISTS playlists;
