ALTER TABLE user_settings
    DROP COLUMN IF EXISTS playlist_auto_advance,
    DROP COLUMN IF EXISTS playlist_countdown_seconds;
