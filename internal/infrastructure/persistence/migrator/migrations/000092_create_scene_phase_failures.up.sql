-- Consecutive failures of a processing phase on a scene. A job failure of the phase adds one
-- (shutdown interruptions and cancellations do not count), a success deletes the row. The size
-- and modification time of the file at the last failure let a replaced file start again from
-- zero. Once failures reach processing.failing_threshold on an unchanged file, the "missing" bulk
-- runs and the automatic retries skip the (scene, phase). Bookkeeping only: no file is touched.
CREATE TABLE scene_phase_failures (
    id BIGSERIAL PRIMARY KEY,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    phase VARCHAR(30) NOT NULL,
    failures INTEGER NOT NULL DEFAULT 1,
    last_error TEXT NOT NULL DEFAULT '',
    last_failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    file_size BIGINT,
    file_mod_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_scene_phase_failures_scene_phase UNIQUE (scene_id, phase)
);

CREATE INDEX idx_scene_phase_failures_phase_failures ON scene_phase_failures (phase, failures);

-- Number of times a job was put back to pending at startup after an interruption while it ran
-- (server shutdown or crash): it is put back once, a second interruption leaves it failed.
ALTER TABLE job_history ADD COLUMN IF NOT EXISTS startup_requeues INTEGER NOT NULL DEFAULT 0;

-- The startup re-queue only takes the latest job of a (scene, phase)
CREATE INDEX IF NOT EXISTS idx_job_history_scene_phase_created ON job_history (scene_id, phase, created_at);
