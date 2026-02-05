ALTER TABLE user_settings
ADD COLUMN sort_preferences JSONB NOT NULL
DEFAULT '{"actors":"name_asc","studios":"name_asc","markers":"label_asc","actor_scenes":"","studio_scenes":""}'::jsonb;
