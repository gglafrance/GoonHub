ALTER TABLE user_scene_markers
    ADD COLUMN end_timestamp INTEGER NULL,
    ADD CONSTRAINT chk_user_scene_markers_end_after_start
        CHECK (end_timestamp IS NULL OR end_timestamp > timestamp);

ALTER TABLE user_settings
    ADD COLUMN marker_loop BOOLEAN NOT NULL DEFAULT FALSE;
