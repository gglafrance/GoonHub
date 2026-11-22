-- Mode of the thumbnail scrubber strip under the player: free (scrolled independently of
-- playback, click to seek) or centered (like Stash: the centre line is the playback position,
-- dragging or wheeling the strip seeks the video)
ALTER TABLE user_settings
    ADD COLUMN scrubber_mode VARCHAR(10) NOT NULL DEFAULT 'free'
        CONSTRAINT chk_user_settings_scrubber_mode CHECK (scrubber_mode IN ('free', 'centered'));
