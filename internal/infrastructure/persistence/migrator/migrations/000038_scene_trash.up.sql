-- Add trash tracking column to scenes
ALTER TABLE scenes ADD COLUMN trashed_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX idx_scenes_trashed_at ON scenes(trashed_at) WHERE trashed_at IS NOT NULL;

-- Create app_settings singleton table for application-wide settings
CREATE TABLE IF NOT EXISTS app_settings (
    id INTEGER PRIMARY KEY DEFAULT 1,
    trash_retention_days INTEGER NOT NULL DEFAULT 7,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT app_settings_singleton CHECK (id = 1)
);
INSERT INTO app_settings (id, trash_retention_days) VALUES (1, 7) ON CONFLICT DO NOTHING;

-- Add scenes:trash permission
INSERT INTO permissions (name, description, created_at)
VALUES ('scenes:trash', 'Move scenes to trash', NOW())
ON CONFLICT (name) DO NOTHING;

-- Grant scenes:trash to admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin' AND p.name = 'scenes:trash'
ON CONFLICT DO NOTHING;

-- Grant scenes:trash to moderator role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'moderator' AND p.name = 'scenes:trash'
ON CONFLICT DO NOTHING;
