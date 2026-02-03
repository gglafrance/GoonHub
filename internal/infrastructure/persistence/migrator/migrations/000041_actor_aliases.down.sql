DROP INDEX IF EXISTS idx_actors_aliases;
ALTER TABLE actors DROP COLUMN IF EXISTS aliases;
