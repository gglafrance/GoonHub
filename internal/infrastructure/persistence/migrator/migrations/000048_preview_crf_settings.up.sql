ALTER TABLE processing_config
    ADD COLUMN marker_preview_crf INTEGER NOT NULL DEFAULT 32,
    ADD COLUMN scene_preview_crf INTEGER NOT NULL DEFAULT 27;
