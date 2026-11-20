-- Ordered, comma-separated criteria used to pick the best variant of a duplicate group
-- (empty = default: resolution,duration,bitrate,size_small)
ALTER TABLE duplication_config
    ADD COLUMN best_selection_criteria TEXT NOT NULL DEFAULT '';
