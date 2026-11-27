-- Dated "o" (jizz) events. user_scene_jizzed.count stays the per-scene total; each event
-- records when one happened (UI increments and imported Stash o_history).
CREATE TABLE user_scene_jizz_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    jizzed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_scene_jizz_events UNIQUE (user_id, scene_id, jizzed_at)
);

CREATE INDEX idx_user_scene_jizz_events_user_time ON user_scene_jizz_events (user_id, jizzed_at);
CREATE INDEX idx_user_scene_jizz_events_scene ON user_scene_jizz_events (scene_id);
