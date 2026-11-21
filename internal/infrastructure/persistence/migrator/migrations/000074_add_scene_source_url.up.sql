-- Where a scene comes from: the web page it was downloaded from (empty = unknown)
ALTER TABLE scenes
    ADD COLUMN source_url TEXT NOT NULL DEFAULT '';

-- Before this column, downloaders (Goongrabber) put the page URL in the
-- description. A description that is nothing but one http(s) URL becomes the
-- source, and the description is emptied. Descriptions with any other text
-- are left alone. Spaces, tabs and line breaks around the URL are trimmed.
UPDATE scenes
SET source_url = btrim(description, E' \t\r\n'),
    description = ''
WHERE btrim(description, E' \t\r\n') ~* '^https?://[^[:space:]]+$';
