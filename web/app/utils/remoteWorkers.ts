// Display helpers of the remote workers settings page.
import type { RemotePhase, RemoteWorker, RoutingMode, WorkerStatus } from '~/types/workers';

export const REMOTE_PHASES: { id: RemotePhase; label: string; icon: string }[] = [
    { id: 'sprites', label: 'Sprites', icon: 'heroicons:squares-2x2' },
    { id: 'animated_thumbnails', label: 'Animated thumbnails', icon: 'heroicons:film' },
    { id: 'fingerprint', label: 'Fingerprint', icon: 'heroicons:finger-print' },
];

export const ROUTING_MODES: { id: RoutingMode; label: string; hint: string }[] = [
    { id: 'local', label: 'Local', hint: 'Only the server pools' },
    { id: 'both', label: 'Both', hint: 'Server pools and workers compete' },
    { id: 'remote', label: 'Remote', hint: 'Only remote workers (no local processing)' },
];

export function phaseLabel(phase: string): string {
    return REMOTE_PHASES.find((p) => p.id === phase)?.label ?? phase;
}

export const WORKER_STATUS: Record<WorkerStatus, { label: string; dot: string; text: string }> = {
    online: { label: 'Online', dot: 'bg-emerald-400', text: 'text-emerald-400' },
    offline: { label: 'Offline', dot: 'bg-amber-400', text: 'text-amber-400' },
    disabled: { label: 'Disabled', dot: 'bg-white/30', text: 'text-dim' },
    revoked: { label: 'Token revoked', dot: 'bg-lava', text: 'text-lava' },
    never_connected: { label: 'Never connected', dot: 'bg-white/20', text: 'text-dim' },
};

export function workerStatus(status: string) {
    return WORKER_STATUS[status as WorkerStatus] ?? WORKER_STATUS.offline;
}

export function formatBytes(bytes: number): string {
    if (!bytes || bytes < 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024;
        unit++;
    }
    return `${value >= 10 || unit === 0 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

// Share of the finished attempts that failed or were lost (expired leases), in percent.
export function failureRate(
    w: Pick<RemoteWorker, 'jobs_completed' | 'jobs_failed' | 'leases_expired'>,
): number {
    const total = w.jobs_completed + w.jobs_failed + w.leases_expired;
    if (total === 0) return 0;
    return Math.round(((w.jobs_failed + w.leases_expired) / total) * 100);
}

// Phases a worker can actually take: its capabilities and what the admin allows.
export function effectivePhases(
    w: Pick<RemoteWorker, 'allowed_phases' | 'capabilities'>,
): RemotePhase[] {
    const caps = new Set(w.capabilities?.phases ?? []);
    return (w.allowed_phases ?? []).filter((p) => caps.has(p));
}

// Total slots a worker advertises.
export function totalSlots(w: Pick<RemoteWorker, 'capabilities'>): number {
    return Object.values(w.capabilities?.slots ?? {}).reduce((sum, n) => sum + (n ?? 0), 0);
}

// Phases routed to the remote workers only for which no online worker can take jobs: their
// jobs wait.
export function unservedRemotePhases(
    routing: Partial<Record<RemotePhase, RoutingMode>>,
    workers: RemoteWorker[],
): RemotePhase[] {
    return REMOTE_PHASES.map((p) => p.id).filter(
        (phase) =>
            routing[phase] === 'remote' &&
            !workers.some((w) => w.status === 'online' && effectivePhases(w).includes(phase)),
    );
}
