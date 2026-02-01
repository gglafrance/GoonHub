-- Rollback migration: Rename scenes back to videos throughout the database

-- Revert permission names from scenes:* back to videos:*
UPDATE permissions SET name = 'videos:view' WHERE name = 'scenes:view';
UPDATE permissions SET name = 'videos:upload' WHERE name = 'scenes:upload';
UPDATE permissions SET name = 'videos:delete' WHERE name = 'scenes:delete';
UPDATE permissions SET name = 'videos:reprocess' WHERE name = 'scenes:reprocess';

-- Drop indexes on new columns
DROP INDEX IF EXISTS idx_scenes_origin;
DROP INDEX IF EXISTS idx_scenes_type;

-- Rename indexes back (reverse order of up migration)
ALTER INDEX idx_user_scene_view_counts_scene_id RENAME TO idx_user_video_view_counts_video_id;
ALTER INDEX idx_user_scene_view_counts_user_id RENAME TO idx_user_video_view_counts_user_id;

ALTER INDEX idx_user_scene_markers_timestamp RENAME TO idx_user_video_markers_timestamp;
ALTER INDEX idx_user_scene_markers_user_scene RENAME TO idx_user_video_markers_user_video;

ALTER INDEX idx_user_scene_watches_user_date RENAME TO idx_user_video_watches_user_date;
ALTER INDEX idx_user_scene_watches_user_scene RENAME TO idx_user_video_watches_user_video;

ALTER INDEX idx_user_scene_jizzed_scene RENAME TO idx_user_video_jizzed_video;

ALTER INDEX idx_user_scene_likes_scene RENAME TO idx_user_video_likes_video;

ALTER INDEX idx_user_scene_ratings_scene RENAME TO idx_user_video_ratings_video;

ALTER INDEX idx_scene_actors_actor_id RENAME TO idx_video_actors_actor_id;
ALTER INDEX idx_scene_actors_scene_id RENAME TO idx_video_actors_video_id;

ALTER INDEX idx_scene_tags_tag_id RENAME TO idx_video_tags_tag_id;
ALTER INDEX idx_scene_tags_scene_id RENAME TO idx_video_tags_video_id;

ALTER INDEX idx_scenes_deleted_at RENAME TO idx_videos_deleted_at;

-- Rename columns back in job_history
ALTER TABLE job_history RENAME COLUMN scene_title TO video_title;
ALTER TABLE job_history RENAME COLUMN scene_id TO video_id;

-- Rename foreign key columns back (scene_id -> video_id)
ALTER TABLE user_scene_view_counts RENAME COLUMN scene_id TO video_id;
ALTER TABLE user_scene_markers RENAME COLUMN scene_id TO video_id;
ALTER TABLE user_scene_watches RENAME COLUMN scene_id TO video_id;
ALTER TABLE user_scene_jizzed RENAME COLUMN scene_id TO video_id;
ALTER TABLE user_scene_likes RENAME COLUMN scene_id TO video_id;
ALTER TABLE user_scene_ratings RENAME COLUMN scene_id TO video_id;
ALTER TABLE scene_actors RENAME COLUMN scene_id TO video_id;
ALTER TABLE scene_tags RENAME COLUMN scene_id TO video_id;

-- Rename junction/interaction tables back
ALTER TABLE user_scene_view_counts RENAME TO user_video_view_counts;
ALTER TABLE user_scene_markers RENAME TO user_video_markers;
ALTER TABLE user_scene_watches RENAME TO user_video_watches;
ALTER TABLE user_scene_jizzed RENAME TO user_video_jizzed;
ALTER TABLE user_scene_likes RENAME TO user_video_likes;
ALTER TABLE user_scene_ratings RENAME TO user_video_ratings;
ALTER TABLE scene_actors RENAME TO video_actors;
ALTER TABLE scene_tags RENAME TO video_tags;

-- Remove new columns
ALTER TABLE scenes DROP COLUMN type;
ALTER TABLE scenes DROP COLUMN origin;

-- Rename main table back
ALTER TABLE scenes RENAME TO videos;
