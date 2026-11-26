-- User profile pages (docs/PROFILES.md): profile fields, avatar and banner, the favorite
-- videos and favorite markers showcases, and the block layout. Everything cascades on user
-- deletion. What a viewer sees is decided by the privacy settings (user_privacy_settings).

-- One row per user who edited their profile; no row = the defaults (no display name, no
-- bio, initials avatar, no banner, default layout).
CREATE TABLE user_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name VARCHAR(64) NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    -- Avatar files <user_id>_<size>.jpg in profiles.image_dir/avatars; the version (Unix ms of
    -- the upload) is appended to the URLs. 0 = no avatar (initials).
    avatar_version BIGINT NOT NULL DEFAULT 0,
    -- Banner: none, the thumbnail of a scene, or an uploaded image (files
    -- <user_id>_<size>.jpg in profiles.image_dir/banners, versioned like the avatar).
    banner_type VARCHAR(16) NOT NULL DEFAULT 'none',
    banner_scene_id BIGINT REFERENCES scenes(id) ON DELETE SET NULL,
    banner_version BIGINT NOT NULL DEFAULT 0,
    -- Ordered blocks [{type, visible, locked_placeholder, config}], validated by the server
    -- (core/profile_blocks.go). NULL = the default layout.
    layout JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_profiles_banner_type CHECK (banner_type IN ('none', 'scene', 'upload')),
    CONSTRAINT chk_user_profiles_bio_length CHECK (char_length(bio) <= 5000),
    CONSTRAINT chk_user_profiles_layout_array CHECK (layout IS NULL OR jsonb_typeof(layout) = 'array')
);
CREATE INDEX idx_user_profiles_banner_scene_id ON user_profiles (banner_scene_id) WHERE banner_scene_id IS NOT NULL;

-- Favorite videos showcase, ordered by position (0 first).
CREATE TABLE user_profile_favorite_scenes (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL,
    PRIMARY KEY (user_id, scene_id),
    CONSTRAINT uni_user_profile_favorite_scenes_position UNIQUE (user_id, position),
    CONSTRAINT chk_user_profile_favorite_scenes_position CHECK (position >= 0)
);
CREATE INDEX idx_user_profile_favorite_scenes_scene_id ON user_profile_favorite_scenes (scene_id);

-- Favorite markers showcase (the owner's markers or markers shared with them), ordered.
CREATE TABLE user_profile_favorite_markers (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    marker_id INTEGER NOT NULL REFERENCES user_scene_markers(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL,
    PRIMARY KEY (user_id, marker_id),
    CONSTRAINT uni_user_profile_favorite_markers_position UNIQUE (user_id, position),
    CONSTRAINT chk_user_profile_favorite_markers_position CHECK (position >= 0)
);
CREATE INDEX idx_user_profile_favorite_markers_marker_id ON user_profile_favorite_markers (marker_id);
