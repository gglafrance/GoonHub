CREATE TABLE trigger_config (
    id SERIAL PRIMARY KEY,
    phase VARCHAR(20) NOT NULL UNIQUE,
    trigger_type VARCHAR(20) NOT NULL DEFAULT 'on_import',
    after_phase VARCHAR(20),
    cron_expression VARCHAR(100),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_phase CHECK (phase IN ('metadata', 'thumbnail', 'sprites')),
    CONSTRAINT valid_trigger_type CHECK (trigger_type IN ('on_import', 'after_job', 'manual', 'scheduled')),
    CONSTRAINT valid_after_phase CHECK (after_phase IS NULL OR after_phase IN ('metadata', 'thumbnail', 'sprites')),
    CONSTRAINT after_phase_required CHECK (
        (trigger_type = 'after_job' AND after_phase IS NOT NULL) OR
        (trigger_type != 'after_job')
    ),
    CONSTRAINT cron_required CHECK (
        (trigger_type = 'scheduled' AND cron_expression IS NOT NULL) OR
        (trigger_type != 'scheduled')
    ),
    CONSTRAINT no_self_reference CHECK (phase != after_phase)
);

-- Default: current behavior (metadata on import, others after metadata)
INSERT INTO trigger_config (phase, trigger_type, after_phase) VALUES
    ('metadata', 'on_import', NULL),
    ('thumbnail', 'after_job', 'metadata'),
    ('sprites', 'manual', NULL);
