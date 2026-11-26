-- Mouse wheel over the thumbnail scrubbers: scrolls the strip (free mode) or seeks (centered
-- mode) when on; off by default so the wheel scrolls the page
ALTER TABLE user_settings
    ADD COLUMN scrubber_wheel BOOLEAN NOT NULL DEFAULT false;
