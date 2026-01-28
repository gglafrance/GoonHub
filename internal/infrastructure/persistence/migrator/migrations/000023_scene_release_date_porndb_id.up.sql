ALTER TABLE videos ADD COLUMN release_date DATE;
ALTER TABLE videos ADD COLUMN porndb_scene_id TEXT DEFAULT '' NOT NULL;
