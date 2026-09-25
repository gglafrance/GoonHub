-- Shorts: what the search page's Shorts filter starts on. Idempotent so it can
-- be re-applied.
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS shorts_search_default TEXT NOT NULL DEFAULT 'hide';
