-- Where page controls are shown on paginated pages: top, bottom or both
ALTER TABLE user_settings
    ADD COLUMN pagination_position VARCHAR(10) NOT NULL DEFAULT 'bottom';
