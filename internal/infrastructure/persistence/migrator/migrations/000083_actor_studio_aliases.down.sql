DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('actors:merge', 'studios:merge')
);
DELETE FROM permissions WHERE name IN ('actors:merge', 'studios:merge');

DROP INDEX IF EXISTS idx_studios_name_lower;
DROP INDEX IF EXISTS idx_actors_name_lower;

ALTER TABLE actors ALTER COLUMN aliases DROP NOT NULL;

DROP INDEX IF EXISTS idx_studios_aliases;
ALTER TABLE studios DROP COLUMN IF EXISTS aliases;
