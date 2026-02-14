-- Revert scene_id/scene_title back to video_id/video_title
ALTER TABLE dead_letter_queue RENAME COLUMN scene_id TO video_id;
ALTER TABLE dead_letter_queue RENAME COLUMN scene_title TO video_title;

DROP INDEX IF EXISTS idx_dlq_scene_id;
CREATE INDEX idx_dlq_video_id ON dead_letter_queue (video_id);
