CREATE TABLE processing_config (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    max_frame_dimension_sm INTEGER NOT NULL DEFAULT 320,
    max_frame_dimension_lg INTEGER NOT NULL DEFAULT 1280,
    frame_quality_sm INTEGER NOT NULL DEFAULT 85,
    frame_quality_lg INTEGER NOT NULL DEFAULT 85,
    frame_quality_sprites INTEGER NOT NULL DEFAULT 75,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
