-- Migration: Rename videos to scenes throughout the database
-- This migration renames all video-related tables and columns to use "scene" terminology

-- Rename main table
ALTER TABLE videos RENAME TO scenes;

-- Add new columns for origin and type
ALTER TABLE scenes ADD COLUMN origin VARCHAR(100);
ALTER TABLE scenes ADD COLUMN type VARCHAR(50);

-- Rename junction/interaction tables
ALTER TABLE video_tags RENAME TO scene_tags;
ALTER TABLE video_actors RENAME TO scene_actors;
ALTER TABLE user_video_ratings RENAME TO user_scene_ratings;
ALTER TABLE user_video_likes RENAME TO user_scene_likes;
ALTER TABLE user_video_jizzed RENAME TO user_scene_jizzed;
ALTER TABLE user_video_watches RENAME TO user_scene_watches;
ALTER TABLE user_video_markers RENAME TO user_scene_markers;
ALTER TABLE user_video_view_counts RENAME TO user_scene_view_counts;

-- Rename foreign key columns (video_id -> scene_id)
ALTER TABLE scene_tags RENAME COLUMN video_id TO scene_id;
ALTER TABLE scene_actors RENAME COLUMN video_id TO scene_id;
ALTER TABLE user_scene_ratings RENAME COLUMN video_id TO scene_id;
ALTER TABLE user_scene_likes RENAME COLUMN video_id TO scene_id;
ALTER TABLE user_scene_jizzed RENAME COLUMN video_id TO scene_id;
ALTER TABLE user_scene_watches RENAME COLUMN video_id TO scene_id;
ALTER TABLE user_scene_markers RENAME COLUMN video_id TO scene_id;
ALTER TABLE user_scene_view_counts RENAME COLUMN video_id TO scene_id;

-- Rename video_id in job_history
ALTER TABLE job_history RENAME COLUMN video_id TO scene_id;
ALTER TABLE job_history RENAME COLUMN video_title TO scene_title;

-- Rename indexes on main table
ALTER INDEX idx_videos_deleted_at RENAME TO idx_scenes_deleted_at;

-- Rename indexes on scene_tags (formerly video_tags)
ALTER INDEX idx_video_tags_video_id RENAME TO idx_scene_tags_scene_id;
ALTER INDEX idx_video_tags_tag_id RENAME TO idx_scene_tags_tag_id;

-- Rename indexes on scene_actors (formerly video_actors)
ALTER INDEX idx_video_actors_video_id RENAME TO idx_scene_actors_scene_id;
ALTER INDEX idx_video_actors_actor_id RENAME TO idx_scene_actors_actor_id;

-- Rename indexes on user_scene_ratings (formerly user_video_ratings)
ALTER INDEX idx_user_video_ratings_video RENAME TO idx_user_scene_ratings_scene;

-- Rename indexes on user_scene_likes (formerly user_video_likes)
ALTER INDEX idx_user_video_likes_video RENAME TO idx_user_scene_likes_scene;

-- Rename indexes on user_scene_jizzed (formerly user_video_jizzed)
ALTER INDEX idx_user_video_jizzed_video RENAME TO idx_user_scene_jizzed_scene;

-- Rename indexes on user_scene_watches (formerly user_video_watches)
ALTER INDEX idx_user_video_watches_user_video RENAME TO idx_user_scene_watches_user_scene;
ALTER INDEX idx_user_video_watches_user_date RENAME TO idx_user_scene_watches_user_date;

-- Rename indexes on user_scene_markers (formerly user_video_markers)
ALTER INDEX idx_user_video_markers_user_video RENAME TO idx_user_scene_markers_user_scene;
ALTER INDEX idx_user_video_markers_timestamp RENAME TO idx_user_scene_markers_timestamp;

-- Rename indexes on user_scene_view_counts (formerly user_video_view_counts)
ALTER INDEX idx_user_video_view_counts_user_id RENAME TO idx_user_scene_view_counts_user_id;
ALTER INDEX idx_user_video_view_counts_video_id RENAME TO idx_user_scene_view_counts_scene_id;

-- Create indexes on new columns for filtering
CREATE INDEX idx_scenes_origin ON scenes(origin) WHERE origin IS NOT NULL;
CREATE INDEX idx_scenes_type ON scenes(type) WHERE type IS NOT NULL;

-- Update permission names from videos:* to scenes:*
UPDATE permissions SET name = 'scenes:view' WHERE name = 'videos:view';
UPDATE permissions SET name = 'scenes:upload' WHERE name = 'videos:upload';
UPDATE permissions SET name = 'scenes:delete' WHERE name = 'videos:delete';
UPDATE permissions SET name = 'scenes:reprocess' WHERE name = 'videos:reprocess';
