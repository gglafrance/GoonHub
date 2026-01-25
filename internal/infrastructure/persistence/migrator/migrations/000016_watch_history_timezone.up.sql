-- Change timestamp columns to timestamptz to properly handle timezones
ALTER TABLE user_video_watches
    ALTER COLUMN watched_at TYPE TIMESTAMPTZ,
    ALTER COLUMN created_at TYPE TIMESTAMPTZ,
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ;
