-- Nothing to restore: the cleared segments were wrong, and the up migration only removed them
-- (the server recomputes them from the stored fingerprints).
SELECT 1;
