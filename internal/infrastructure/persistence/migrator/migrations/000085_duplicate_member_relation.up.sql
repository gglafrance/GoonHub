-- Relation of a duplicate group member to the scene it was matched against (ref_scene_id),
-- derived from the matching segments and the scene durations (see core.ClassifyDuplicateRelation):
-- identical, contained (this scene is a part of the reference), contains (the reference is a
-- part of this scene), partial, or unknown (no segment yet). Existing rows start as unknown and
-- are classified by the application at startup from their stored segments.
ALTER TABLE duplicate_group_members
    ADD COLUMN relation VARCHAR(10) NOT NULL DEFAULT 'unknown'
        CONSTRAINT chk_duplicate_group_members_relation
        CHECK (relation IN ('identical', 'contained', 'contains', 'partial', 'unknown'));

-- Group filters look up the members of a given relation
CREATE INDEX idx_duplicate_group_members_group_relation
    ON duplicate_group_members (group_id, relation);
