-- share_markers: the user's markers are visible (read-only) to every user
ALTER TABLE users ADD COLUMN share_markers BOOLEAN NOT NULL DEFAULT FALSE;
