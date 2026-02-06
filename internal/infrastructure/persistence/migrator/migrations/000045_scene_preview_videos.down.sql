ALTER TABLE scenes DROP COLUMN IF EXISTS preview_video_path;

ALTER TABLE processing_config
  DROP COLUMN IF EXISTS scene_preview_enabled,
  DROP COLUMN IF EXISTS scene_preview_segments,
  DROP COLUMN IF EXISTS scene_preview_segment_duration;
