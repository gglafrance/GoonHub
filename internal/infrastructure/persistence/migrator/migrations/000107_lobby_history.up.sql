-- Goon2gether: the items a lobby played (docs/GOON2GETHER.md "History"), bounded per lobby
-- by goon2gether.history_max, deleted with the lobby. The lobby keeps when its current item
-- first played and whether it reached its end, so a play that goes on across an unload or a
-- restart is recorded once, when the item changes.

CREATE TABLE lobby_history (
    id BIGSERIAL PRIMARY KEY,
    lobby_id BIGINT NOT NULL REFERENCES lobbies(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    start_s DOUBLE PRECISION,
    end_s DOUBLE PRECISION,
    label VARCHAR(100),
    added_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    added_by_anon INTEGER,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NOT NULL,
    outcome VARCHAR(10) NOT NULL,
    CONSTRAINT chk_lobby_history_outcome CHECK (outcome IN ('completed', 'skipped')),
    CONSTRAINT chk_lobby_history_start CHECK (start_s IS NULL OR start_s >= 0),
    CONSTRAINT chk_lobby_history_range CHECK (end_s IS NULL OR end_s > COALESCE(start_s, 0)),
    CONSTRAINT chk_lobby_history_added_by_anon CHECK (added_by_anon IS NULL OR added_by_anon > 0)
);

CREATE INDEX idx_lobby_history_lobby_id ON lobby_history (lobby_id, id DESC);
CREATE INDEX idx_lobby_history_scene_id ON lobby_history (scene_id);

ALTER TABLE lobbies
    ADD COLUMN play_started_at TIMESTAMPTZ,
    ADD COLUMN play_completed BOOLEAN NOT NULL DEFAULT FALSE;
