// Remote workers (goonhub-worker), admin API /api/v1/admin/workers.

export type RemotePhase = 'sprites' | 'animated_thumbnails' | 'fingerprint';

export type RoutingMode = 'local' | 'remote' | 'both';

export type WorkerStatus = 'online' | 'offline' | 'disabled' | 'revoked' | 'never_connected';

export type ConnectionMode = 'pull' | 'push';

export interface WorkerCapabilities {
    phases: RemotePhase[] | null;
    slots: Partial<Record<RemotePhase, number>> | null;
    input_modes: string[] | null;
    accel: string;
    hwaccels?: string[] | null;
    encoders?: string[] | null;
    audio_fingerprint: boolean;
    ffmpeg_version?: string;
}

export interface WorkerPathMapping {
    server_prefix: string;
    local_prefix: string;
}

export interface WorkerJob {
    job_id: string;
    phase: RemotePhase;
    scene_id: number;
    scene_title: string;
    progress: number;
    stage: string;
    input_mode: string;
    attempt: number;
    generation: number;
    started_at: string;
    lease_expires_at?: string;
}

export interface RemoteWorker {
    id: number;
    name: string;
    enabled: boolean;
    draining: boolean;
    connection_mode: ConnectionMode;
    push_url: string;
    server_url: string;
    allowed_phases: RemotePhase[];
    max_concurrency: number;
    version: string;
    hostname: string;
    os: string;
    arch: string;
    free_disk_bytes: number;
    worker_draining: boolean;
    last_seen_at: string | null;
    registered_at: string | null;
    jobs_completed: number;
    jobs_failed: number;
    leases_expired: number;
    leases_released: number;
    last_error: string;
    last_error_at: string | null;
    created_at: string;
    status: WorkerStatus;
    has_token: boolean;
    capabilities: WorkerCapabilities;
    path_mappings: WorkerPathMapping[];
    jobs: WorkerJob[];
}

export interface WorkerRouting {
    enabled: boolean;
    effective: Record<RemotePhase, RoutingMode>;
    defaults: Record<RemotePhase, RoutingMode>;
    overrides: Partial<Record<RemotePhase, RoutingMode>>;
}

export interface WorkersSettings {
    heartbeat_interval_sec: number;
    lease_duration_sec: number;
    offline_after_sec: number;
    public_url: string;
}

export interface WorkersListResponse {
    enabled: boolean;
    workers: RemoteWorker[];
    routing: WorkerRouting;
    settings: WorkersSettings;
}

export interface CreatedWorker {
    worker: RemoteWorker;
    token: string;
}

export interface WorkerInput {
    name?: string;
    enabled?: boolean;
    draining?: boolean;
    connection_mode?: ConnectionMode;
    push_url?: string;
    allowed_phases?: RemotePhase[];
    max_concurrency?: number;
    revoke_token?: boolean;
}
