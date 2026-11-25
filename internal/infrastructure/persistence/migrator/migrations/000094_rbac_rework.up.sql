-- Roles and permissions rework (docs/RBAC.md): parent roles, allow/deny settings, per-user
-- overrides, superuser and system roles, permission groups, users.role foreign key.

-- Roles: system roles cannot be deleted, a superuser role has every permission
ALTER TABLE roles ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE roles ADD COLUMN is_superuser BOOLEAN NOT NULL DEFAULT FALSE;

INSERT INTO roles (name, description) VALUES ('admin', 'Full system access') ON CONFLICT (name) DO NOTHING;
INSERT INTO roles (name, description) VALUES ('user', 'Regular viewer') ON CONFLICT (name) DO NOTHING;
UPDATE roles SET is_system = TRUE, is_superuser = TRUE WHERE name = 'admin';
UPDATE roles SET is_system = TRUE WHERE name = 'user';

-- Parent roles: a role inherits the permissions of its parents (several, no cycle: checked
-- by the application)
CREATE TABLE role_parents (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    parent_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, parent_id),
    CONSTRAINT chk_role_parents_not_self CHECK (role_id <> parent_id)
);
CREATE INDEX idx_role_parents_parent_id ON role_parents (parent_id);

-- Role settings: allow or deny (no row = inherit). Existing rows are grants.
ALTER TABLE role_permissions ADD COLUMN effect VARCHAR(5) NOT NULL DEFAULT 'allow';
ALTER TABLE role_permissions ADD CONSTRAINT chk_role_permissions_effect CHECK (effect IN ('allow', 'deny'));

-- Permission group, shown by the UI (synced from the catalog at startup)
ALTER TABLE permissions ADD COLUMN group_name VARCHAR(50) NOT NULL DEFAULT '';

-- Per-user overrides: beat the role's effective value
CREATE TABLE user_permission_overrides (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    effect VARCHAR(5) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, permission_id),
    CONSTRAINT chk_user_permission_overrides_effect CHECK (effect IN ('allow', 'deny'))
);
CREATE INDEX idx_user_permission_overrides_permission_id ON user_permission_overrides (permission_id);

-- users.role references roles.name: a renamed role follows, a role held by users cannot be
-- deleted. Roles held by users but missing from roles (no permission at all) are recreated
-- empty so that the constraint holds.
INSERT INTO roles (name, description)
SELECT DISTINCT u.role, 'Recreated by migration 094 (held by users)'
FROM users u
WHERE NOT EXISTS (SELECT 1 FROM roles r WHERE r.name = u.role);

ALTER TABLE users ADD CONSTRAINT fk_users_role
    FOREIGN KEY (role) REFERENCES roles(name) ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX idx_users_role ON users (role);

-- The user role becomes the "regular viewer" profile: no upload. It keeps scenes:view and
-- the playlists; the catalog sync gives it scenes:share and markers:create (from scenes:view).
DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE name = 'user')
  AND permission_id = (SELECT id FROM permissions WHERE name = 'scenes:upload');

-- The seeded description of the user role mentioned upload, which it no longer has
UPDATE roles SET description = 'Browse and watch scenes, markers and playlists'
WHERE name = 'user' AND description = 'Basic view and upload access';
