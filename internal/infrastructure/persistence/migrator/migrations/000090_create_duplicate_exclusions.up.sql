-- Pairs of scenes an admin marked as "not duplicates" by removing one of them from a duplicate
-- group (POST /admin/duplicates/:id/exclude). Fingerprint matching ignores a match between the
-- two scenes of a pair, so a later fingerprint run never puts them back in the same group.
-- The pair is stored once, ordered (scene_id_a < scene_id_b).
CREATE TABLE duplicate_exclusions (
    scene_id_a BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    scene_id_b BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scene_id_a, scene_id_b),
    CONSTRAINT chk_duplicate_exclusions_ordered CHECK (scene_id_a < scene_id_b)
);

CREATE INDEX idx_duplicate_exclusions_scene_id_b ON duplicate_exclusions(scene_id_b);
