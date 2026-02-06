ALTER TABLE user_settings
    ADD COLUMN playlist_auto_advance VARCHAR(20) NOT NULL DEFAULT 'countdown',
    ADD COLUMN playlist_countdown_seconds INTEGER NOT NULL DEFAULT 5;
