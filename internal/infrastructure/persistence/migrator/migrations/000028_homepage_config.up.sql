ALTER TABLE user_settings
ADD COLUMN homepage_config JSONB NOT NULL DEFAULT '{
    "show_upload": true,
    "sections": [{
        "id": "default-latest",
        "type": "latest",
        "title": "Latest Uploads",
        "enabled": true,
        "limit": 12,
        "order": 0,
        "sort": "created_at_desc",
        "config": {}
    }]
}'::jsonb;
