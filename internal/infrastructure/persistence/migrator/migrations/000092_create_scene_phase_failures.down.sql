DROP INDEX IF EXISTS idx_job_history_scene_phase_created;
ALTER TABLE job_history DROP COLUMN IF EXISTS startup_requeues;
DROP TABLE IF EXISTS scene_phase_failures;
