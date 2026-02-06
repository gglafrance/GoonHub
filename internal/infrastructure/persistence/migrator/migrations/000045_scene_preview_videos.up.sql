ALTER TABLE scenes ADD COLUMN preview_video_path VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE processing_config
  ADD COLUMN scene_preview_enabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN scene_preview_segments INTEGER NOT NULL DEFAULT 12,
  ADD COLUMN scene_preview_segment_duration REAL NOT NULL DEFAULT 1.0;
