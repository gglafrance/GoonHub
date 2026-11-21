-- Touch gestures on the video player: on/off and the double-tap seek step in seconds
ALTER TABLE user_settings
    ADD COLUMN player_gestures BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN gesture_seek_step INTEGER NOT NULL DEFAULT 10;
