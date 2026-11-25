-- Server-wide marker labels (docs/MARKER_LABELS.md).
--
-- Marker labels were free text typed by each user (user_scene_markers.label) and the default
-- tags of a label were per user (marker_label_tags.user_id). They become a server referential:
-- marker_labels (one row per label, name unique ignoring case), markers point to it with
-- label_id, and the default tags of a label are the same for everyone.
--
-- user_scene_markers.label stays as a copy of the label name kept by the database (trigger
-- below): label_id is the source of truth, the text column lets the existing listings and
-- filters read the name without a join.

CREATE TABLE marker_labels (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    color VARCHAR(7) NOT NULL DEFAULT '#FFFFFF',
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_marker_labels_name_not_blank CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX idx_marker_labels_name_lower ON marker_labels (lower(name));

-- The labels in use: those of the markers and those having default tags, spaces trimmed,
-- merged ignoring case. The canonical spelling is the most used one (by markers; ties: the
-- first in byte order, so "Kiss" before "kiss").
INSERT INTO marker_labels (name)
SELECT DISTINCT ON (lower(name)) name
FROM (
    SELECT btrim(label) AS name, COUNT(*) AS uses
    FROM user_scene_markers
    WHERE btrim(COALESCE(label, '')) <> ''
    GROUP BY btrim(label)
    UNION ALL
    SELECT btrim(label), 0
    FROM marker_label_tags
    WHERE btrim(label) <> ''
    GROUP BY btrim(label)
) spellings
ORDER BY lower(name), uses DESC, name COLLATE "C" ASC;

-- Color: the most used marker color of the label; creation date: its first marker (or default
-- tag)
UPDATE marker_labels ml
SET color = g.color, created_at = g.first_at, updated_at = g.first_at
FROM (
    SELECT lower(btrim(label)) AS k,
           mode() WITHIN GROUP (ORDER BY color) AS color,
           MIN(created_at) AS first_at
    FROM user_scene_markers
    WHERE btrim(COALESCE(label, '')) <> '' AND color ~ '^#[0-9A-Fa-f]{6}$'
    GROUP BY lower(btrim(label))
) g
WHERE lower(ml.name) = g.k;

UPDATE marker_labels ml
SET created_at = g.first_at, updated_at = g.first_at
FROM (
    SELECT lower(btrim(label)) AS k, MIN(created_at) AS first_at
    FROM marker_label_tags
    GROUP BY lower(btrim(label))
) g
WHERE lower(ml.name) = g.k AND g.first_at < ml.created_at;

-- Markers point to their label. A marker without label (empty or spaces only) has no label_id
-- (NULL is allowed) and an empty label text. Deleting a label leaves its markers without label.
ALTER TABLE user_scene_markers
    ADD COLUMN label_id BIGINT REFERENCES marker_labels(id) ON DELETE SET NULL;

UPDATE user_scene_markers m
SET label_id = ml.id, label = ml.name
FROM marker_labels ml
WHERE btrim(COALESCE(m.label, '')) <> '' AND lower(ml.name) = lower(btrim(m.label));

UPDATE user_scene_markers SET label = '' WHERE label_id IS NULL AND label IS DISTINCT FROM '';

ALTER TABLE user_scene_markers
    ALTER COLUMN label SET DEFAULT '',
    ALTER COLUMN label SET NOT NULL;

CREATE INDEX idx_user_scene_markers_label_id ON user_scene_markers (label_id);

-- Default tags become global: the union of the users' default tags of each label.
ALTER TABLE marker_label_tags
    ADD COLUMN label_id BIGINT REFERENCES marker_labels(id) ON DELETE CASCADE;

UPDATE marker_label_tags t
SET label_id = ml.id
FROM marker_labels ml
WHERE lower(ml.name) = lower(btrim(t.label));

DELETE FROM marker_label_tags WHERE label_id IS NULL;

DELETE FROM marker_label_tags a
USING marker_label_tags b
WHERE a.label_id = b.label_id AND a.tag_id = b.tag_id AND a.id > b.id;

-- Dropping the columns drops the (user_id, label, tag_id) unique constraint and the
-- (user_id, label) index
ALTER TABLE marker_label_tags
    DROP COLUMN user_id,
    DROP COLUMN label,
    ALTER COLUMN label_id SET NOT NULL;

ALTER TABLE marker_label_tags
    ADD CONSTRAINT uq_marker_label_tags_label_tag UNIQUE (label_id, tag_id);

-- Every marker gets the default tags of its label it lacks (as tags from the label). A tag the
-- marker already has keeps its origin, so a tag set by hand is never turned into a label tag.
INSERT INTO marker_tags (marker_id, tag_id, is_from_label)
SELECT m.id, lt.tag_id, TRUE
FROM user_scene_markers m
JOIN marker_label_tags lt ON lt.label_id = m.label_id
ON CONFLICT (marker_id, tag_id) DO NOTHING;

-- user_scene_markers.label follows label_id. A write that sets label_id (or clears it) wins;
-- a write that only changes the label text (legacy writers) is resolved to the label of that
-- name, ignoring case and surrounding spaces, created when missing. The application checks
-- the permission to create labels before writing: the creation here only keeps the copy
-- consistent.
CREATE FUNCTION user_scene_markers_sync_label() RETURNS trigger AS $$
DECLARE
    wanted TEXT;
BEGIN
    IF (TG_OP = 'INSERT' AND NEW.label_id IS NULL)
        OR (TG_OP = 'UPDATE' AND NEW.label_id IS NOT DISTINCT FROM OLD.label_id
            AND NEW.label IS DISTINCT FROM OLD.label) THEN
        wanted := left(btrim(COALESCE(NEW.label, '')), 100);
        IF wanted = '' THEN
            NEW.label_id := NULL;
        ELSE
            INSERT INTO marker_labels (name) VALUES (wanted)
            ON CONFLICT ((lower(name))) DO NOTHING;
            SELECT id INTO NEW.label_id FROM marker_labels WHERE lower(name) = lower(wanted);
        END IF;
    END IF;

    IF NEW.label_id IS NULL THEN
        NEW.label := '';
    ELSE
        SELECT name INTO NEW.label FROM marker_labels WHERE id = NEW.label_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_user_scene_markers_sync_label
    BEFORE INSERT OR UPDATE ON user_scene_markers
    FOR EACH ROW EXECUTE FUNCTION user_scene_markers_sync_label();

-- A renamed label renames the copy on its markers (updated_at is not touched: it versions the
-- marker thumbnails).
CREATE FUNCTION marker_labels_propagate_name() RETURNS trigger AS $$
BEGIN
    UPDATE user_scene_markers SET label = NEW.name
    WHERE label_id = NEW.id AND label IS DISTINCT FROM NEW.name;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_marker_labels_propagate_name
    AFTER UPDATE OF name ON marker_labels
    FOR EACH ROW WHEN (OLD.name IS DISTINCT FROM NEW.name)
    EXECUTE FUNCTION marker_labels_propagate_name();
