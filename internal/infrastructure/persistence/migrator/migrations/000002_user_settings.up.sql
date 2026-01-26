CREATE TABLE user_settings (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL,
    autoplay BOOLEAN NOT NULL DEFAULT false,
    default_volume INTEGER NOT NULL DEFAULT 100,
    loop BOOLEAN NOT NULL DEFAULT true,
    videos_per_page INTEGER NOT NULL DEFAULT 20,
    default_sort_order VARCHAR(50) NOT NULL DEFAULT 'created_at_desc',
    CONSTRAINT fk_user_settings_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT uni_user_settings_user_id UNIQUE (user_id)
);
