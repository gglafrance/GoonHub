DROP INDEX IF EXISTS idx_marker_tags_tag_id;
DROP INDEX IF EXISTS idx_marker_tags_marker_id;
DROP TABLE IF EXISTS marker_tags;

DROP INDEX IF EXISTS idx_marker_label_tags_tag_id;
DROP INDEX IF EXISTS idx_marker_label_tags_user_label;
DROP TABLE IF EXISTS marker_label_tags;
