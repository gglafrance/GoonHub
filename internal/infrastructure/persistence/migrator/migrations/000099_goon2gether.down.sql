-- The emote image files in goon2gether.emote_dir are left on disk.
DROP TABLE IF EXISTS emotes;
ALTER TABLE IF EXISTS lobbies DROP CONSTRAINT IF EXISTS fk_lobbies_current_item;
DROP TABLE IF EXISTS lobby_queue_items;
DROP TABLE IF EXISTS lobby_members;
DROP TABLE IF EXISTS lobbies;
