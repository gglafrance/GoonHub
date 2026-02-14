CREATE TABLE duplication_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    audio_density_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.50,
    audio_min_hashes INTEGER NOT NULL DEFAULT 40,
    visual_hamming_max INTEGER NOT NULL DEFAULT 5,
    visual_min_frames INTEGER NOT NULL DEFAULT 20,
    delta_tolerance INTEGER NOT NULL DEFAULT 2,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO duplication_config (id) VALUES (1);
