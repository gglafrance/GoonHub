ALTER TABLE IF EXISTS lobby_queue_items
    DROP CONSTRAINT IF EXISTS chk_lobby_queue_items_added_by_anon,
    DROP COLUMN IF EXISTS added_by_anon;

DROP INDEX IF EXISTS idx_lobby_members_anon_number;

ALTER TABLE IF EXISTS lobby_members
    DROP CONSTRAINT IF EXISTS chk_lobby_members_anon,
    DROP COLUMN IF EXISTS anon_number,
    DROP COLUMN IF EXISTS anonymous;

ALTER TABLE IF EXISTS lobbies
    DROP CONSTRAINT IF EXISTS chk_lobbies_anon_seq,
    DROP CONSTRAINT IF EXISTS chk_lobbies_auto_next,
    DROP COLUMN IF EXISTS anon_seq,
    DROP COLUMN IF EXISTS auto_next;
