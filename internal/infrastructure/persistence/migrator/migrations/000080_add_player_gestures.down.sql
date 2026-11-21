ALTER TABLE user_settings
    DROP COLUMN IF EXISTS gesture_seek_step,
    DROP COLUMN IF EXISTS player_gestures;
