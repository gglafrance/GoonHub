import { describe, expect, test } from 'bun:test';
import type { RemoteWorker } from '../app/types/workers';
import {
    effectivePhases,
    failureRate,
    formatBytes,
    phaseLabel,
    totalSlots,
    unservedRemotePhases,
    workerStatus,
} from '../app/utils/remoteWorkers';

const worker = (over: Partial<RemoteWorker> = {}): RemoteWorker => ({
    id: 1,
    name: 'pc',
    enabled: true,
    draining: false,
    connection_mode: 'pull',
    push_url: '',
    server_url: '',
    allowed_phases: ['sprites', 'animated_thumbnails', 'fingerprint'],
    max_concurrency: 0,
    version: '',
    hostname: '',
    os: '',
    arch: '',
    free_disk_bytes: 0,
    worker_draining: false,
    last_seen_at: null,
    registered_at: null,
    jobs_completed: 0,
    jobs_failed: 0,
    leases_expired: 0,
    leases_released: 0,
    last_error: '',
    last_error_at: null,
    created_at: '',
    status: 'online',
    has_token: true,
    capabilities: {
        phases: ['sprites', 'fingerprint'],
        slots: { sprites: 1, fingerprint: 2 },
        input_modes: ['direct', 'indirect'],
        accel: 'nvidia',
        audio_fingerprint: true,
    },
    path_mappings: [],
    jobs: [],
    ...over,
});

describe('remote workers display helpers', () => {
    test('effective phases are the allowed phases the worker can run', () => {
        expect(effectivePhases(worker())).toEqual(['sprites', 'fingerprint']);
        expect(effectivePhases(worker({ allowed_phases: ['animated_thumbnails'] }))).toEqual([]);
        expect(
            effectivePhases(worker({ capabilities: { ...worker().capabilities, phases: null } })),
        ).toEqual([]);
    });

    test('failure rate counts failures and expired leases', () => {
        expect(failureRate(worker())).toBe(0);
        expect(failureRate(worker({ jobs_completed: 8, jobs_failed: 1, leases_expired: 1 }))).toBe(
            20,
        );
    });

    test('slots and bytes', () => {
        expect(totalSlots(worker())).toBe(3);
        expect(
            totalSlots(worker({ capabilities: { ...worker().capabilities, slots: null } })),
        ).toBe(0);
        expect(formatBytes(0)).toBe('0 B');
        expect(formatBytes(1536)).toBe('1.5 KB');
        expect(formatBytes(52_000_000_000)).toBe('48 GB');
    });

    test('labels and statuses', () => {
        expect(phaseLabel('animated_thumbnails')).toBe('Animated thumbnails');
        expect(phaseLabel('metadata')).toBe('metadata');
        expect(workerStatus('online').label).toBe('Online');
        expect(workerStatus('weird').label).toBe('Offline');
    });

    test('remote-only phases without an online worker are reported', () => {
        const routing = {
            sprites: 'remote',
            animated_thumbnails: 'remote',
            fingerprint: 'both',
        } as const;
        expect(unservedRemotePhases(routing, [worker()])).toEqual(['animated_thumbnails']);
        expect(unservedRemotePhases(routing, [worker({ status: 'offline' })])).toEqual([
            'sprites',
            'animated_thumbnails',
        ]);
        expect(unservedRemotePhases({ sprites: 'local' }, [])).toEqual([]);
    });
});
