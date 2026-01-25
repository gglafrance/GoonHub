-- actors table
CREATE TABLE IF NOT EXISTS actors (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    name VARCHAR(255) NOT NULL,
    image_url VARCHAR(512),
    gender VARCHAR(50),
    birthday DATE,
    date_of_death DATE,
    astrology VARCHAR(50),
    birthplace VARCHAR(255),
    ethnicity VARCHAR(100),
    nationality VARCHAR(100),
    career_start_year INTEGER,
    career_end_year INTEGER,
    height_cm INTEGER,
    weight_kg INTEGER,
    measurements VARCHAR(50),
    cupsize VARCHAR(10),
    hair_color VARCHAR(50),
    eye_color VARCHAR(50),
    tattoos TEXT,
    piercings TEXT,
    fake_boobs BOOLEAN NOT NULL DEFAULT FALSE,
    same_sex_only BOOLEAN NOT NULL DEFAULT FALSE
);

-- video_actors junction table
CREATE TABLE IF NOT EXISTS video_actors (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    actor_id BIGINT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
    UNIQUE(video_id, actor_id)
);

CREATE INDEX idx_actors_uuid ON actors(uuid);
CREATE INDEX idx_actors_name ON actors(name);
CREATE INDEX idx_actors_deleted_at ON actors(deleted_at);
CREATE INDEX idx_video_actors_video_id ON video_actors(video_id);
CREATE INDEX idx_video_actors_actor_id ON video_actors(actor_id);
