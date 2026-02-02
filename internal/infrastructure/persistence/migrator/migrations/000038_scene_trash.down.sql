-- Remove scenes:trash permission from roles
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name = 'scenes:trash'
);

-- Remove scenes:trash permission
DELETE FROM permissions WHERE name = 'scenes:trash';

-- Drop app_settings table
DROP TABLE IF EXISTS app_settings;

-- Remove trash column and index from scenes
DROP INDEX IF EXISTS idx_scenes_trashed_at;
ALTER TABLE scenes DROP COLUMN IF EXISTS trashed_at;
