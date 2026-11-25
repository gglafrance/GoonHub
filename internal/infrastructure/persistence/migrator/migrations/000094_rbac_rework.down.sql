-- The user role gets the upload back
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name = 'scenes:upload'
ON CONFLICT DO NOTHING;

ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_role;
DROP INDEX IF EXISTS idx_users_role;

DROP TABLE IF EXISTS user_permission_overrides;

ALTER TABLE permissions DROP COLUMN IF EXISTS group_name;

-- Without the effect column every row is a grant: denials go away first
DELETE FROM role_permissions WHERE effect = 'deny';
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS chk_role_permissions_effect;
ALTER TABLE role_permissions DROP COLUMN IF EXISTS effect;

DROP TABLE IF EXISTS role_parents;

ALTER TABLE roles DROP COLUMN IF EXISTS is_superuser;
ALTER TABLE roles DROP COLUMN IF EXISTS is_system;
