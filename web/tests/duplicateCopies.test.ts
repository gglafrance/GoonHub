import { describe, expect, test } from 'bun:test';
import {
    buildCopyQuery,
    defaultCopyFilters,
    describeCopyBulkResult,
    fileName,
    parseSizeMB,
    selectionSummary,
    sizeRangeError,
} from '../app/utils/duplicateCopies';

const fmt = (b: number) => `${b}B`;

describe('parseSizeMB', () => {
    test('reads positive numbers, with a comma or a dot', () => {
        expect(parseSizeMB('12')).toBe(12);
        expect(parseSizeMB(' 1,5 ')).toBe(1.5);
        expect(parseSizeMB(250)).toBe(250);
    });
    test('empty, zero, negative or invalid give no bound', () => {
        for (const raw of ['', '  ', '0', '-3', 'abc', null, undefined]) {
            expect(parseSizeMB(raw)).toBeNull();
        }
    });
});

describe('buildCopyQuery', () => {
    test('defaults send only the page', () => {
        expect(buildCopyQuery(defaultCopyFilters(), 1, 20).toString()).toBe('page=1&limit=20');
    });
    test('filters and sort, sizes converted from MB to bytes', () => {
        const q = buildCopyQuery(
            {
                storagePathId: 3,
                search: ' stash ',
                minSizeMB: 1,
                maxSizeMB: 2.5,
                sort: 'copies',
            },
            2,
        );
        expect(Object.fromEntries(q)).toEqual({
            page: '2',
            storage_path_id: '3',
            q: 'stash',
            min_size: '1048576',
            max_size: '2621440',
            sort_by: 'copies',
        });
    });
    test('without page for the select-all IDs', () => {
        expect(buildCopyQuery({ ...defaultCopyFilters(), sort: 'size' }).toString()).toBe(
            'sort_by=size',
        );
    });
});

test('sizeRangeError', () => {
    expect(sizeRangeError({ ...defaultCopyFilters(), minSizeMB: 5, maxSizeMB: 2 })).not.toBeNull();
    expect(sizeRangeError({ ...defaultCopyFilters(), minSizeMB: 2, maxSizeMB: 5 })).toBeNull();
    expect(sizeRangeError({ ...defaultCopyFilters(), minSizeMB: 5 })).toBeNull();
});

test('selectionSummary sums the loaded copies and counts the others as unknown', () => {
    const known = new Map([
        [1, { size: 100 }],
        [2, { size: 50 }],
    ]);
    expect(selectionSummary(new Set([1, 2, 9]), known)).toEqual({
        count: 3,
        size: 150,
        unknown: 1,
    });
    expect(selectionSummary(new Set(), known)).toEqual({ count: 0, size: 0, unknown: 0 });
});

describe('describeCopyBulkResult', () => {
    test('forget', () => {
        expect(
            describeCopyBulkResult('forget', { done: [1], failed: [], freed_bytes: 0 }, fmt),
        ).toBe('1 copy forgotten (files kept on disk)');
    });
    test('delete with refusals', () => {
        expect(
            describeCopyBulkResult(
                'delete',
                {
                    done: [1, 2],
                    failed: [{ copy_id: 3, error: 'storage path "x" is read-only' }],
                    freed_bytes: 300,
                },
                fmt,
            ),
        ).toBe(
            '2 file(s) deleted, 300B freed; 1 refused or failed (e.g. storage path "x" is read-only)',
        );
    });
});

test('fileName', () => {
    expect(fileName('/media/a/b/video.mp4')).toBe('video.mp4');
    expect(fileName('C:\\media\\video.mkv')).toBe('video.mkv');
});
