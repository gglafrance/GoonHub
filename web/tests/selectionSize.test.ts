import { describe, expect, test } from 'bun:test';
import {
    rememberSceneSizes,
    selectionSizeLabel,
    summarizeSelectionSize,
    type SceneSizeCache,
} from '../app/utils/selectionSize';

const GB = 1024 ** 3;
const fmt = (bytes: number) => `${bytes} B`;

describe('rememberSceneSizes', () => {
    test('records sizes and reports changes', () => {
        const cache: SceneSizeCache = new Map();
        expect(
            rememberSceneSizes(cache, [
                { id: 1, size: 100 },
                { id: 2, size: 200 },
            ]),
        ).toBe(true);
        expect(rememberSceneSizes(cache, [{ id: 1, size: 100 }])).toBe(false);
        expect(rememberSceneSizes(cache, [{ id: 1, size: 150 }])).toBe(true);
        expect(cache.get(1)).toBe(150);
        expect(cache.get(2)).toBe(200);
    });

    test('stores missing, negative or invalid sizes as unknown (0)', () => {
        const cache: SceneSizeCache = new Map();
        rememberSceneSizes(cache, [
            { id: 1 },
            { id: 2, size: null },
            { id: 3, size: -5 },
            { id: 4, size: Number.NaN },
        ]);
        expect([...cache.values()]).toEqual([0, 0, 0, 0]);
    });
});

describe('summarizeSelectionSize', () => {
    const cache: SceneSizeCache = new Map([
        [1, 2 * GB],
        [2, 3 * GB],
        [3, 0],
    ]);

    test('sums the known sizes of the selection only', () => {
        expect(summarizeSelectionSize([1, 2], cache)).toEqual({
            total: 5 * GB,
            known: 2,
            unknown: 0,
            missing: 0,
        });
    });

    test('counts unknown (0) and not loaded sizes apart', () => {
        expect(summarizeSelectionSize(new Set([1, 3, 9]), cache)).toEqual({
            total: 2 * GB,
            known: 1,
            unknown: 1,
            missing: 1,
        });
    });

    test('empty selection', () => {
        expect(summarizeSelectionSize([], cache)).toEqual({
            total: 0,
            known: 0,
            unknown: 0,
            missing: 0,
        });
    });
});

describe('selectionSizeLabel', () => {
    test('formats the total when every size is known', () => {
        expect(selectionSizeLabel({ total: 300, known: 2, unknown: 0, missing: 0 }, fmt)).toEqual({
            size: '300 B',
            note: null,
        });
    });

    test('notes the scenes whose size is unknown', () => {
        expect(selectionSizeLabel({ total: 300, known: 2, unknown: 3, missing: 0 }, fmt)).toEqual({
            size: '300 B',
            note: 'size of 3 unknown',
        });
    });

    test('hides a partial total while sizes are loading', () => {
        const summary = { total: 300, known: 2, unknown: 0, missing: 4 };
        expect(selectionSizeLabel(summary, fmt, true)).toBeNull();
        // Loaded but still missing (deleted scene, failed request): counted as unknown
        expect(selectionSizeLabel(summary, fmt, false)).toEqual({
            size: '300 B',
            note: 'size of 4 unknown',
        });
    });

    test('shows the total right away when nothing is missing, even while loading', () => {
        expect(
            selectionSizeLabel({ total: 300, known: 2, unknown: 0, missing: 0 }, fmt, true),
        ).toEqual({ size: '300 B', note: null });
    });

    test('says so when no size is known', () => {
        expect(selectionSizeLabel({ total: 0, known: 0, unknown: 2, missing: 1 }, fmt)).toEqual({
            size: 'size unknown',
            note: null,
        });
        expect(selectionSizeLabel({ total: 0, known: 0, unknown: 0, missing: 0 }, fmt)).toBeNull();
    });

    test('works with the app size formatter', () => {
        const formatSize = (bytes: number) => {
            const k = 1024;
            const units = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + units[i];
        };
        const summary = summarizeSelectionSize(
            [1, 2],
            new Map([
                [1, 10 * GB],
                [2, 8.4 * GB],
            ]),
        );
        expect(selectionSizeLabel(summary, formatSize)?.size).toBe('18.4 GB');
    });
});
