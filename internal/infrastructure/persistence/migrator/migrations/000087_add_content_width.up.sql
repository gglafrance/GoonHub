-- Maximum width of the scene list pages: default (the usual width), wide or full (edge to edge,
-- with the usual side padding). The scene grids add columns as the width grows.
ALTER TABLE user_settings
    ADD COLUMN content_width VARCHAR(10) NOT NULL DEFAULT 'default'
        CONSTRAINT chk_user_settings_content_width CHECK (content_width IN ('default', 'wide', 'full'));
