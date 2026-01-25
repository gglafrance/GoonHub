-- Revert to timestamp without timezone
ALTER TABLE user_video_watches
    ALTER COLUMN watched_at TYPE TIMESTAMP,
    ALTER COLUMN created_at TYPE TIMESTAMP,
    ALTER COLUMN updated_at TYPE TIMESTAMP;
