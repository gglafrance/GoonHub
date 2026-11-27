import { describe, expect, test } from 'bun:test';
import {
    COMPARE_PAGE_SIZE,
    compareGridColumns,
    compareOffsetOf,
    compareOffsetRange,
    comparePageCount,
    comparePageOf,
    comparePageSlice,
    compareSegmentStyle,
    compareTileTime,
} from '../app/utils/duplicateCompare';

describe('compare grid', () => {
    test('2 tiles in 2 columns, 3-4 in 2x2, 5-9 in 3 columns', () => {
        expect(compareGridColumns(1)).toBe(1);
        expect(compareGridColumns(2)).toBe(2);
        expect(compareGridColumns(3)).toBe(2);
        expect(compareGridColumns(4)).toBe(2);
        expect(compareGridColumns(5)).toBe(3);
        expect(compareGridColumns(9)).toBe(3);
    });

    test('pages of 9 videos', () => {
        expect(COMPARE_PAGE_SIZE).toBe(9);
        expect(comparePageCount(0)).toBe(1);
        expect(comparePageCount(9)).toBe(1);
        expect(comparePageCount(148)).toBe(17);
        expect(comparePageOf(8)).toBe(0);
        expect(comparePageOf(9)).toBe(1);
        const items = Array.from({ length: 20 }, (_, i) => i);
        expect(comparePageSlice(items, 2)).toEqual([18, 19]);
        expect(comparePageSlice(items, 0)).toHaveLength(9);
    });
});

describe('compare sync', () => {
    const clip = { duration: 300, match_start: 0, match_end: 300 };
    const compilation = { duration: 3600, match_start: 1200, match_end: 1500 };
    const noSegment = { duration: 100 };

    test('a shared offset keeps every tile on the same moment of its segment', () => {
        expect(compareTileTime(clip, 30)).toBe(30);
        expect(compareTileTime(compilation, 30)).toBe(1230);
        expect(compareTileTime(noSegment, 30)).toBe(30);
        expect(compareOffsetOf(compilation, 1230)).toBe(30);
    });

    test('tile times stay inside the video', () => {
        expect(compareTileTime(clip, -10)).toBe(0);
        expect(compareTileTime(noSegment, 500)).toBe(100);
        expect(compareTileTime({ duration: 0, match_start: 5 }, 10)).toBe(15);
    });

    test('offset range spans every tile', () => {
        expect(compareOffsetRange([clip, compilation, noSegment])).toEqual({
            min: -1200,
            max: 2400,
        });
        expect(compareOffsetRange([])).toEqual({ min: 0, max: 0 });
    });

    test('segment on the timeline', () => {
        expect(compareSegmentStyle(compilation)).toEqual({
            left: '33.33333333333333%',
            width: '8.333333333333332%',
        });
        expect(compareSegmentStyle(noSegment)).toBeNull();
    });
});
