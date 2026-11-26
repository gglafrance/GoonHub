-- Per-account layout of the two header bars (docs/NAVIGATION.md): where each item goes (bar and
-- zone, or a folder), the user's folders and the hidden items. One row per customized user; no
-- row = the default layout (today's header), so a user who never customized follows the
-- defaults of later versions. The layout is validated by the application (known item ids, no
-- duplicates, folder limits) and the items the user lacks the permission for are left out.
CREATE TABLE user_nav_layouts (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    layout JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_nav_layouts_layout CHECK (jsonb_typeof(layout) = 'object')
);
