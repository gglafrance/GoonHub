-- Back to per-user free text labels and per-user default tags.
--
-- Lost on the way down: the spelling variants merged by the up migration (markers keep the
-- canonical name), the label colors and descriptions, the labels without markers, and the
-- default tags of a label that no user has markers with. The default tags of a label are
-- given to every user who has markers with that label. The marker tags added by the up
-- migration (union of the users' default tags) stay on the markers.

DROP TRIGGER IF EXISTS trg_marker_labels_propagate_name ON marker_labels;
DROP FUNCTION IF EXISTS marker_labels_propagate_name();
DROP TRIGGER IF EXISTS trg_user_scene_markers_sync_label ON user_scene_markers;
DROP FUNCTION IF EXISTS user_scene_markers_sync_label();

ALTER TABLE marker_label_tags DROP CONSTRAINT IF EXISTS uq_marker_label_tags_label_tag;

ALTER TABLE marker_label_tags
    ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN label VARCHAR(100);

INSERT INTO marker_label_tags (user_id, label, tag_id, label_id, created_at)
SELECT DISTINCT m.user_id, ml.name, lt.tag_id, lt.label_id, lt.created_at
FROM marker_label_tags lt
JOIN marker_labels ml ON ml.id = lt.label_id
JOIN user_scene_markers m ON m.label_id = lt.label_id
WHERE lt.user_id IS NULL;

DELETE FROM marker_label_tags WHERE user_id IS NULL;

ALTER TABLE marker_label_tags
    DROP COLUMN label_id,
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN label SET NOT NULL;

ALTER TABLE marker_label_tags
    ADD CONSTRAINT marker_label_tags_user_id_label_tag_id_key UNIQUE (user_id, label, tag_id);

CREATE INDEX idx_marker_label_tags_user_label ON marker_label_tags (user_id, label);

DROP INDEX IF EXISTS idx_user_scene_markers_label_id;

ALTER TABLE user_scene_markers
    DROP COLUMN label_id,
    ALTER COLUMN label DROP NOT NULL,
    ALTER COLUMN label DROP DEFAULT;

DROP TABLE marker_labels;
