ALTER TABLE users ADD COLUMN last_login_at TIMESTAMPTZ;

CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name VARCHAR(50) NOT NULL,
    description VARCHAR(255),
    CONSTRAINT uni_roles_name UNIQUE (name)
);

CREATE TABLE permissions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name VARCHAR(100) NOT NULL,
    description VARCHAR(255),
    CONSTRAINT uni_permissions_name UNIQUE (name)
);

CREATE TABLE role_permissions (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    CONSTRAINT uni_role_permissions UNIQUE (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_role_id ON role_permissions (role_id);

-- Seed roles
INSERT INTO roles (name, description) VALUES
    ('admin', 'Full system access'),
    ('moderator', 'Video management access'),
    ('user', 'Basic view and upload access');

-- Seed permissions
INSERT INTO permissions (name, description) VALUES
    ('videos:view', 'View and stream videos'),
    ('videos:upload', 'Upload new videos'),
    ('videos:delete', 'Delete videos'),
    ('videos:reprocess', 'Reprocess videos'),
    ('users:manage', 'Manage users'),
    ('users:create', 'Create new users'),
    ('users:delete', 'Delete users'),
    ('roles:manage', 'Manage roles and permissions'),
    ('settings:manage', 'Manage application settings');

-- Admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'admin';

-- Moderator gets video permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'moderator' AND p.name IN ('videos:view','videos:upload','videos:delete','videos:reprocess');

-- User gets basic permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name IN ('videos:view','videos:upload');
