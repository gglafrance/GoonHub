-- Aliases for studios, the same way as actors (migration 041): other names under which
-- a studio is known. Resolution by name (import clients, PornDB matching, the resolve
-- endpoint) also matches them, case-insensitively.
ALTER TABLE studios ADD COLUMN aliases TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_studios_aliases ON studios USING GIN (aliases);

-- actors.aliases was nullable: same shape as studios.aliases
UPDATE actors SET aliases = '{}' WHERE aliases IS NULL;
ALTER TABLE actors ALTER COLUMN aliases SET NOT NULL;

-- Case-insensitive lookups by exact name (resolve, duplicate check)
CREATE INDEX idx_actors_name_lower ON actors (lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_studios_name_lower ON studios (lower(name)) WHERE deleted_at IS NULL;

-- Merging an actor or a studio into another one: admin and moderator
INSERT INTO permissions (name, description, created_at) VALUES
    ('actors:merge', 'Merge an actor into another one', NOW()),
    ('studios:merge', 'Merge a studio into another one', NOW())
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('admin', 'moderator') AND p.name IN ('actors:merge', 'studios:merge')
ON CONFLICT DO NOTHING;
