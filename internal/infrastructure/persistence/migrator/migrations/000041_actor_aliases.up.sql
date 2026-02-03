ALTER TABLE actors ADD COLUMN aliases TEXT[] DEFAULT '{}';
CREATE INDEX idx_actors_aliases ON actors USING GIN (aliases);
