-- Social graph (docs/SOCIAL.md): one-way follows, friendships by request and acceptance,
-- per-user privacy settings. Everything cascades on user deletion.

-- Follows: follower_id follows followed_id (no approval). Two users following each other
-- are "mutuals".
CREATE TABLE user_follows (
    follower_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followed_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, followed_id),
    CONSTRAINT chk_user_follows_not_self CHECK (follower_id <> followed_id)
);
CREATE INDEX idx_user_follows_followed_id ON user_follows (followed_id, created_at DESC);

-- Friendships: one row per pair, stored with the smaller id first (a pair cannot be stored
-- twice). The user_friends view gives both directions.
CREATE TABLE user_friendships (
    user_low_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_high_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_low_id, user_high_id),
    CONSTRAINT chk_user_friendships_ordered CHECK (user_low_id < user_high_id)
);
CREATE INDEX idx_user_friendships_user_high_id ON user_friendships (user_high_id);

-- Both directions of every friendship: (user_id, friend_id) for each side
CREATE VIEW user_friends AS
    SELECT user_low_id AS user_id, user_high_id AS friend_id, created_at FROM user_friendships
    UNION ALL
    SELECT user_high_id AS user_id, user_low_id AS friend_id, created_at FROM user_friendships;

-- Pending friend requests (accepted, declined and cancelled ones are deleted)
CREATE TABLE friend_requests (
    id BIGSERIAL PRIMARY KEY,
    sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_friend_requests_pair UNIQUE (sender_id, receiver_id),
    CONSTRAINT chk_friend_requests_not_self CHECK (sender_id <> receiver_id)
);
CREATE INDEX idx_friend_requests_receiver_id ON friend_requests (receiver_id, created_at DESC);

-- Privacy choices: who can see an information item of the user (catalog in code,
-- core/privacy_catalog.go). No row = the catalog default of the item.
CREATE TABLE user_privacy_settings (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item VARCHAR(64) NOT NULL,
    audience VARCHAR(16) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, item),
    CONSTRAINT chk_user_privacy_settings_audience CHECK (audience IN ('public', 'followers', 'mutuals', 'friends', 'private'))
);
CREATE INDEX idx_user_privacy_settings_item ON user_privacy_settings (item);

-- Social preferences of a user. No row = defaults (friend requests from everyone).
CREATE TABLE user_social_settings (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    friend_request_policy VARCHAR(16) NOT NULL DEFAULT 'everyone',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_social_settings_friend_request_policy CHECK (friend_request_policy IN ('everyone', 'mutuals', 'nobody'))
);
