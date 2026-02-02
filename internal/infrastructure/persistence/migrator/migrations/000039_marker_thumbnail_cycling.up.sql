-- Add marker thumbnail cycling preference to user_settings
ALTER TABLE user_settings ADD COLUMN marker_thumbnail_cycling BOOLEAN NOT NULL DEFAULT true;
