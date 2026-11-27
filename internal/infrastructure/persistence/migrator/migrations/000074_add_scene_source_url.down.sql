-- Keep the sources: put them back in empty descriptions before dropping the column
UPDATE scenes
SET description = source_url
WHERE description = '' AND source_url <> '';

ALTER TABLE scenes DROP COLUMN IF EXISTS source_url;
