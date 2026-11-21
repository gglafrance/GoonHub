-- Reverts the schema only. The actors and studios merged by the up migration (split by
-- alias, see the up file) stay merged: a merge cannot be undone (the sources stay
-- soft-deleted, their scenes, ratings and likes stay on the target).
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('actors:merge', 'studios:merge')
);
DELETE FROM permissions WHERE name IN ('actors:merge', 'studios:merge');

DROP INDEX IF EXISTS idx_studios_name_lower;
DROP INDEX IF EXISTS idx_actors_name_lower;

ALTER TABLE actors ALTER COLUMN aliases DROP NOT NULL;

DROP INDEX IF EXISTS idx_studios_aliases;
ALTER TABLE studios DROP COLUMN IF EXISTS aliases;
