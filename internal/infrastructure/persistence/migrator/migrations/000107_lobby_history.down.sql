ALTER TABLE IF EXISTS lobbies
    DROP COLUMN IF EXISTS play_completed,
    DROP COLUMN IF EXISTS play_started_at;

DROP TABLE IF EXISTS lobby_history;
