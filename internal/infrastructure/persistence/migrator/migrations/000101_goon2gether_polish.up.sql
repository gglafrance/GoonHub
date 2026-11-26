-- Goon2gether polish (docs/GOON2GETHER.md): auto-next mode of a lobby, anonymous membership
-- of public lobbies (shown as "Anonymous #N" to the other members).

ALTER TABLE lobbies
    ADD COLUMN auto_next VARCHAR(16) NOT NULL DEFAULT 'next',
    ADD COLUMN anon_seq INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT chk_lobbies_auto_next CHECK (auto_next IN ('stop', 'next', 'repeat_one', 'repeat_queue')),
    ADD CONSTRAINT chk_lobbies_anon_seq CHECK (anon_seq >= 0);

ALTER TABLE lobby_members
    ADD COLUMN anonymous BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN anon_number INTEGER,
    ADD CONSTRAINT chk_lobby_members_anon CHECK (
        (anonymous AND anon_number IS NOT NULL AND anon_number > 0)
        OR (NOT anonymous AND anon_number IS NULL)
    );

CREATE UNIQUE INDEX idx_lobby_members_anon_number ON lobby_members (lobby_id, anon_number)
    WHERE anon_number IS NOT NULL;

-- The anonymous number of the member who added the item (NULL: added under their name), so
-- that the identity stays hidden after the member left or the lobby was reloaded
ALTER TABLE lobby_queue_items
    ADD COLUMN added_by_anon INTEGER,
    ADD CONSTRAINT chk_lobby_queue_items_added_by_anon CHECK (added_by_anon IS NULL OR added_by_anon > 0);
