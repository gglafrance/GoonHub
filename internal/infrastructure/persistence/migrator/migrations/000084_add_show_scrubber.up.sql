-- Thumbnail scrubber strip under the player on the watch page (off by default)
ALTER TABLE user_settings
    ADD COLUMN show_scrubber BOOLEAN NOT NULL DEFAULT false;
