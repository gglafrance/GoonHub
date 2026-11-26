ALTER TABLE share_links
    DROP CONSTRAINT IF EXISTS chk_share_links_range,
    DROP CONSTRAINT IF EXISTS chk_share_links_start,
    DROP COLUMN IF EXISTS marker_id,
    DROP COLUMN IF EXISTS label,
    DROP COLUMN IF EXISTS end_s,
    DROP COLUMN IF EXISTS start_s;

ALTER TABLE user_social_settings
    DROP CONSTRAINT IF EXISTS chk_user_social_settings_message_policy,
    DROP COLUMN IF EXISTS message_toasts,
    DROP COLUMN IF EXISTS message_policy;

DROP TABLE IF EXISTS conversation_participants;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
