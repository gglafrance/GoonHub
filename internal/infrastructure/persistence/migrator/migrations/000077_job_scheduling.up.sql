-- Job scheduling: when and in which order the job queue feeder runs each phase.
-- The defaults keep the previous behaviour: every phase runs at any time with its
-- pool size, no global limit, no boost for new scenes.

-- Global settings (singleton)
CREATE TABLE job_schedule_config (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    quiet_hours_start VARCHAR(5) NOT NULL DEFAULT '01:00',
    quiet_hours_end VARCHAR(5) NOT NULL DEFAULT '07:00',
    max_concurrent_jobs INTEGER NOT NULL DEFAULT 0,
    new_scenes_first BOOLEAN NOT NULL DEFAULT false,
    new_scenes_window_hours INTEGER NOT NULL DEFAULT 24,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_quiet_hours_start CHECK (quiet_hours_start ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    CONSTRAINT valid_quiet_hours_end CHECK (quiet_hours_end ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    CONSTRAINT valid_max_concurrent_jobs CHECK (max_concurrent_jobs >= 0),
    CONSTRAINT valid_new_scenes_window_hours CHECK (new_scenes_window_hours BETWEEN 1 AND 720)
);

INSERT INTO job_schedule_config (id) VALUES (1);

-- Per-phase settings
CREATE TABLE job_phase_schedules (
    id SERIAL PRIMARY KEY,
    phase VARCHAR(20) NOT NULL UNIQUE,
    priority INTEGER NOT NULL DEFAULT 0,
    window_enabled BOOLEAN NOT NULL DEFAULT false,
    window_start VARCHAR(5) NOT NULL DEFAULT '01:00',
    window_end VARCHAR(5) NOT NULL DEFAULT '07:00',
    quiet_workers INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_phase_schedule_phase CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint')),
    CONSTRAINT valid_phase_schedule_priority CHECK (priority BETWEEN 0 AND 100),
    CONSTRAINT valid_phase_schedule_window_start CHECK (window_start ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    CONSTRAINT valid_phase_schedule_window_end CHECK (window_end ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    CONSTRAINT valid_phase_schedule_window CHECK (window_start <> window_end),
    CONSTRAINT valid_phase_schedule_quiet_workers CHECK (quiet_workers BETWEEN 0 AND 32)
);

-- Higher priority gets the next free slot when the global limit is reached
INSERT INTO job_phase_schedules (phase, priority) VALUES
    ('metadata', 50),
    ('thumbnail', 40),
    ('animated_thumbnails', 30),
    ('sprites', 20),
    ('fingerprint', 10);
