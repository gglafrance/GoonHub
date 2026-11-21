-- Scene grid density (compact, comfortable or large cards) and infinite scroll instead of
-- page controls on the scene list pages
ALTER TABLE user_settings
    ADD COLUMN grid_density VARCHAR(12) NOT NULL DEFAULT 'comfortable',
    ADD COLUMN infinite_scroll BOOLEAN NOT NULL DEFAULT false;
