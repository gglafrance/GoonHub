-- How scene lists are displayed: a grid of cards or a table, per page type
-- (search, actor, studio, playlist, explorer; missing = grid), and the table columns
ALTER TABLE user_settings
    ADD COLUMN scene_list_view JSONB NOT NULL DEFAULT '{
        "pages": {},
        "columns": ["thumbnail", "title", "duration", "resolution", "file_size", "rating", "views", "studio", "actors", "added_at"]
    }'::jsonb;
