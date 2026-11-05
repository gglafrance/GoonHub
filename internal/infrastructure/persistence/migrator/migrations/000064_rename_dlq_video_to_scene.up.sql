-- Rename legacy video_id/video_title columns to scene_id/scene_title
ALTER TABLE dead_letter_queue RENAME COLUMN video_id TO scene_id;
ALTER TABLE dead_letter_queue RENAME COLUMN video_title TO scene_title;

-- Rename the index to match
DROP INDEX IF EXISTS idx_dlq_video_id;
CREATE INDEX idx_dlq_scene_id ON dead_letter_queue (scene_id);
