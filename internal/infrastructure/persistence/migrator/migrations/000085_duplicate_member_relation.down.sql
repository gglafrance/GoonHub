DROP INDEX IF EXISTS idx_duplicate_group_members_group_relation;

ALTER TABLE duplicate_group_members DROP COLUMN IF EXISTS relation;
