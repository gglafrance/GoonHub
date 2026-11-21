ALTER TABLE user_settings
    DROP COLUMN IF EXISTS infinite_scroll,
    DROP COLUMN IF EXISTS grid_density;
