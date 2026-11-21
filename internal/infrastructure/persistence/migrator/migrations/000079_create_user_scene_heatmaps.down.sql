ALTER TABLE user_settings
    DROP COLUMN IF EXISTS record_replay_heatmap,
    DROP COLUMN IF EXISTS show_replay_heatmap;

DROP TABLE IF EXISTS user_scene_heatmaps;
