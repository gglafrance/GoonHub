ALTER TABLE user_settings
    ADD COLUMN scene_card_config JSONB NOT NULL DEFAULT '{
        "badges": {
            "top_left": {"items": ["rating"], "direction": "vertical"},
            "top_right": {"items": ["watched"], "direction": "vertical"},
            "bottom_left": {"items": [], "direction": "vertical"},
            "bottom_right": {"items": ["duration"], "direction": "horizontal"}
        },
        "content_rows": [
            {"type": "split", "left": "file_size", "right": "added_at"}
        ]
    }'::jsonb;
