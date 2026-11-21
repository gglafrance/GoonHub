-- Background library health tasks (admin "Health" page): the missing-file check, which stats
-- every scene file, and the "fix sizes" task, which re-reads the size of scenes stored with
-- size 0. Both only read the media files; the results live here.
CREATE TABLE library_health_checks (
    id BIGSERIAL PRIMARY KEY,
    kind VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    total INT NOT NULL DEFAULT 0,
    checked INT NOT NULL DEFAULT 0,
    missing INT NOT NULL DEFAULT 0,
    unreadable INT NOT NULL DEFAULT 0,
    fixed INT NOT NULL DEFAULT 0,
    unchanged INT NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_library_health_checks_kind CHECK (kind IN ('missing_files', 'fix_sizes')),
    CONSTRAINT chk_library_health_checks_status CHECK (status IN ('running', 'completed', 'cancelled', 'failed'))
);

CREATE INDEX idx_library_health_checks_kind_started ON library_health_checks(kind, started_at DESC);

-- Files found missing or unreadable by a missing-file check. Only the items of the latest
-- completed check are kept.
CREATE TABLE library_health_check_items (
    id BIGSERIAL PRIMARY KEY,
    check_id BIGINT NOT NULL REFERENCES library_health_checks(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    stored_path TEXT NOT NULL,
    reason VARCHAR(20) NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    CONSTRAINT chk_library_health_check_items_reason CHECK (reason IN ('missing', 'unreadable'))
);

CREATE INDEX idx_library_health_check_items_check_reason ON library_health_check_items(check_id, reason, scene_id);
CREATE INDEX idx_library_health_check_items_scene_id ON library_health_check_items(scene_id);
