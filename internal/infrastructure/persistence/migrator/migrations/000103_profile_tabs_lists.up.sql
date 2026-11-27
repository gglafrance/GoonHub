-- Profile tabs, sections and custom lists (docs/PROFILES.md, "Tabs and sections", "Custom
-- lists").

-- The tabs of a profile [{id, kind, title, icon, slug, visible, sections: [{id, template,
-- columns: [[block]]}]}], validated by the server (core/profile_tabs.go). NULL = derived from
-- the older single block list (layout), itself NULL for the default layout.
ALTER TABLE user_profiles ADD COLUMN tabs JSONB;
ALTER TABLE user_profiles ADD CONSTRAINT chk_user_profiles_tabs_array CHECK (tabs IS NULL OR jsonb_typeof(tabs) = 'array');

-- Custom lists: search criteria (the saved search filters shape) run with the owner's
-- interactions, shown by "custom_list" blocks and browsable on the profile. The audience is
-- the list's own; the privacy items the criteria depend on are checked too.
CREATE TABLE user_profile_lists (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(80) NOT NULL,
    filters JSONB NOT NULL DEFAULT '{}',
    -- optional live source: a saved search of the owner (its current filters are used while
    -- it exists, else the copy in filters)
    saved_search_uuid UUID,
    audience VARCHAR(16) NOT NULL DEFAULT 'public',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_profile_lists_audience CHECK (audience IN ('public', 'followers', 'mutuals', 'friends', 'private')),
    CONSTRAINT chk_user_profile_lists_filters_object CHECK (jsonb_typeof(filters) = 'object'),
    CONSTRAINT chk_user_profile_lists_title CHECK (char_length(title) BETWEEN 1 AND 80)
);
CREATE INDEX idx_user_profile_lists_user_id ON user_profile_lists (user_id);
