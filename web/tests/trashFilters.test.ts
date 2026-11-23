import { describe, expect, test } from 'bun:test';
import {
    activeTrashFilterCount,
    chunkIds,
    describeTrashBulk,
    emptyTrashFilters,
    mergeBulkResults,
    purgeCountdown,
    summarizeTrashSelection,
    trashFilterParams,
} from '../app/utils/trashFilters';

const fmt = (bytes: number) => `${bytes} B`;

describe('trashFilterParams', () => {
    test('only the sort when no filter is set', () => {
        expect(trashFilterParams(emptyTrashFilters(), 'trashed_at', 'desc')).toEqual({
            sort: 'trashed_at',
            order: 'desc',
        });
    });

    test('converts megabytes to bytes and minutes to seconds', () => {
        const f = {
            ...emptyTrashFilters(),
            q: '  beach ',
            storage: 'none',
            minSizeMb: 1.5,
            maxSizeMb: 1024,
            minDurationMin: 2,
            maxDurationMin: 0.5,
            from: '2026-11-01',
            to: '2026-11-02',
            resolution: '1080p',
            codec: 'hevc',
            source: 'duplicates' as const,
        };
        expect(trashFilterParams(f, 'size', 'asc')).toEqual({
            sort: 'size',
            order: 'asc',
            q: 'beach',
            storage_path_id: 'none',
            min_size: String(1.5 * 1024 * 1024),
            max_size: String(1024 * 1024 * 1024),
            min_duration: '120',
            max_duration: '30',
            trashed_from: '2026-11-01',
            trashed_to: '2026-11-02',
            resolution: '1080p',
            codec: 'hevc',
            source: 'duplicates',
        });
    });
});

describe('activeTrashFilterCount', () => {
    test('counts ranges once and ignores the text search', () => {
        expect(activeTrashFilterCount(emptyTrashFilters())).toBe(0);
        expect(
            activeTrashFilterCount({
                ...emptyTrashFilters(),
                q: 'x',
                minSizeMb: 1,
                maxSizeMb: 2,
                to: '2026-01-01',
                codec: 'h264',
            }),
        ).toBe(3);
    });
});

describe('chunkIds and mergeBulkResults', () => {
    test('splits in chunks of the bulk limit', () => {
        const ids = Array.from({ length: 1001 }, (_, i) => i + 1);
        const chunks = chunkIds(ids);
        expect(chunks.map((c) => c.length)).toEqual([500, 500, 1]);
        expect(chunks.flat()).toEqual(ids);
        expect(chunkIds([])).toEqual([]);
    });

    test('merges per-chunk results', () => {
        expect(
            mergeBulkResults([
                { succeeded: [1], failed: [], size: 10 },
                { succeeded: [2, 3], failed: [{ scene_id: 4, error: 'x' }], size: 5 },
            ]),
        ).toEqual({ succeeded: [1, 2, 3], failed: [{ scene_id: 4, error: 'x' }], size: 15 });
    });
});

describe('summarizeTrashSelection', () => {
    test('sums known sizes, counts unknown ones and files kept on disk', () => {
        const sizes = new Map([
            [1, 100],
            [2, 0],
            [3, 50],
        ]);
        expect(summarizeTrashSelection([1, 2, 3, 4, 1], sizes, new Set([3]))).toEqual({
            count: 4,
            size: 150,
            unknownSize: 1,
            keptOnDisk: 1,
        });
    });
});

describe('describeTrashBulk', () => {
    test('reports successes, size and the first failure', () => {
        expect(
            describeTrashBulk(
                'deleted',
                {
                    succeeded: [1, 2],
                    failed: [{ scene_id: 9, error: 'scene is not in trash' }],
                    size: 300,
                },
                fmt,
            ),
        ).toBe('2 videos permanently deleted (300 B), 1 failed (#9: scene is not in trash)');
        expect(describeTrashBulk('restored', { succeeded: [1], failed: [], size: 0 }, fmt)).toBe(
            '1 video restored',
        );
    });
});

describe('purgeCountdown', () => {
    const now = Date.parse('2026-11-23T12:00:00Z');
    test('days, hours and overdue', () => {
        expect(purgeCountdown('2026-11-26T12:00:00Z', now)).toEqual({
            label: 'Purged in 3 days',
            soon: false,
        });
        expect(purgeCountdown('2026-11-24T10:00:00Z', now)).toEqual({
            label: 'Purged in 22 h',
            soon: true,
        });
        expect(purgeCountdown('2026-11-23T11:00:00Z', now)).toEqual({
            label: 'Purge pending',
            soon: true,
        });
        expect(purgeCountdown('not a date', now)).toEqual({ label: '', soon: false });
    });
});
