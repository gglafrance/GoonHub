ALTER TABLE processing_config
    DROP COLUMN IF EXISTS marker_preview_crf,
    DROP COLUMN IF EXISTS scene_preview_crf;
