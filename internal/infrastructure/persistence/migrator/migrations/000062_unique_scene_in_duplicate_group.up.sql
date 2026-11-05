-- Remove duplicate scene memberships (keep lowest group_id)
DELETE FROM duplicate_group_members
WHERE id NOT IN (
    SELECT MIN(id) FROM duplicate_group_members GROUP BY scene_id
);

-- Enforce one group per scene
CREATE UNIQUE INDEX idx_duplicate_group_members_unique_scene
    ON duplicate_group_members(scene_id);
