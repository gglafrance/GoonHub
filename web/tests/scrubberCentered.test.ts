import { describe, expect, test } from 'bun:test';
import {
    SCRUBBER_END_MARGIN,
    SCRUBBER_SEEK_MAX_WAIT_MS,
    SCRUBBER_SEEK_MIN_DELTA,
    centerOffsetForTime,
    centerTime,
    centeredTranslate,
    centeredViewLeft,
    clampCenterOffset,
    dragCenterOffset,
    formatScrubberPreciseTime,
    needsFinalSeek,
    normalizeScrubberMode,
    shouldIssueSeek,
    visibleTileRange,
} from '../app/utils/scrubber';

// 10 second cues: [0,10), [10,20), ...
const cues = (n: number, step = 10) =>
    Array.from({ length: n }, (_, i) => ({ start: i * step, end: (i + 1) * step }));

describe('normalizeScrubberMode', () => {
    test('keeps the known modes, free otherwise', () => {
        expect(normalizeScrubberMode('centered')).toBe('centered');
        expect(normalizeScrubberMode('free')).toBe('free');
        expect(normalizeScrubberMode(undefined)).toBe('free');
        expect(normalizeScrubberMode('Centered')).toBe('free');
    });
});

describe('centre offset <-> time', () => {
    const list = cues(6); // 60 s, 100 px per thumbnail
    test('the time under the centre is linear inside a thumbnail, not snapped', () => {
        expect(centerTime(list, 0, 100, 60)).toBe(0);
        expect(centerTime(list, 37, 100, 60)).toBeCloseTo(3.7, 6);
        expect(centerTime(list, 250, 100, 60)).toBeCloseTo(25, 6);
    });
    test('the offset for a time is the inverse', () => {
        for (const t of [0, 0.5, 9.99, 10, 33.3, 59]) {
            const offset = centerOffsetForTime(list, t, 100);
            expect(centerTime(list, offset, 100, 60)).toBeCloseTo(t, 6);
        }
    });
    test('fine control: a pixel is a tenth of a second here', () => {
        const a = centerTime(list, 400, 100, 60);
        const b = centerTime(list, 401, 100, 60);
        expect(b - a).toBeCloseTo(0.1, 6);
    });
    test('clamped at the start of the scene', () => {
        expect(centerTime(list, -500, 100, 60)).toBe(0);
        expect(centerOffsetForTime(list, -3, 100)).toBe(0);
    });
    test('clamped a hair before the end of the scene', () => {
        expect(centerTime(list, 99999, 100, 60)).toBeCloseTo(60 - SCRUBBER_END_MARGIN, 6);
        expect(centerOffsetForTime(list, 500, 100)).toBe(600);
    });
    test('the duration bounds the time when the last thumbnail goes past it', () => {
        // last cue [50,60) but the scene ends at 55 s
        expect(centerTime(list, 590, 100, 55)).toBeCloseTo(55 - SCRUBBER_END_MARGIN, 6);
    });
    test('unknown duration: the cues bound the time', () => {
        expect(centerTime(list, 99999, 100, 0)).toBe(60);
    });
    test('no cues', () => {
        expect(centerTime([], 120, 100, 60)).toBe(0);
        expect(centerOffsetForTime([], 12, 100)).toBe(0);
    });
});

describe('centered layout', () => {
    test('the content is translated so that the offset is under the centre line', () => {
        expect(centeredTranslate(0, 800)).toBe(400);
        expect(centeredTranslate(1000, 800)).toBe(-600);
    });
    test('visible part of the content, negative before the strip start', () => {
        expect(centeredViewLeft(0, 800)).toBe(-400);
        expect(centeredViewLeft(1000, 800)).toBe(600);
    });
    test('only the thumbnails around the centre are rendered', () => {
        // offset 5000 px in a 100 x 100 px strip, 800 px view: 4600..5400 visible
        const range = visibleTileRange(centeredViewLeft(5000, 800), 800, 100, 100, 2);
        expect(range).toEqual({ first: 44, last: 55 });
        // at the start, the left half of the view is empty
        expect(visibleTileRange(centeredViewLeft(0, 800), 800, 100, 100, 2)).toEqual({
            first: 0,
            last: 5,
        });
    });
    test('clampCenterOffset', () => {
        expect(clampCenterOffset(-1, 600)).toBe(0);
        expect(clampCenterOffset(700, 600)).toBe(600);
        expect(clampCenterOffset(Number.NaN, 600)).toBe(0);
    });
});

describe('dragCenterOffset', () => {
    test('dragging left goes forward, right goes back', () => {
        expect(dragCenterOffset(300, 500, 450, 600)).toBe(350);
        expect(dragCenterOffset(300, 500, 600, 600)).toBe(200);
    });
    test('clamped to the content', () => {
        expect(dragCenterOffset(300, 500, 2000, 600)).toBe(0);
        expect(dragCenterOffset(300, 500, -2000, 600)).toBe(600);
    });
});

describe('seek throttling', () => {
    test('the first seek of a drag goes at once', () => {
        expect(shouldIssueSeek(12, Number.NaN, false, 0)).toBe(true);
    });
    test('a target next to the last seek is skipped', () => {
        expect(shouldIssueSeek(12 + SCRUBBER_SEEK_MIN_DELTA / 2, 12, false, 1000)).toBe(false);
        expect(shouldIssueSeek(12 + SCRUBBER_SEEK_MIN_DELTA * 2, 12, false, 0)).toBe(true);
    });
    test('waits for the running seek, unless it takes too long', () => {
        expect(shouldIssueSeek(20, 12, true, 10)).toBe(false);
        expect(shouldIssueSeek(20, 12, true, SCRUBBER_SEEK_MAX_WAIT_MS)).toBe(true);
    });
    test('never seeks to an invalid time', () => {
        expect(shouldIssueSeek(Number.NaN, 12, false, 1000)).toBe(false);
    });
    test('the end of a drag seeks precisely when needed', () => {
        expect(needsFinalSeek(20, 20, false)).toBe(false);
        expect(needsFinalSeek(20, 20, true)).toBe(true); // last one went to a keyframe
        expect(needsFinalSeek(20, 19.9, false)).toBe(true);
        expect(needsFinalSeek(20, Number.NaN, false)).toBe(true);
        expect(needsFinalSeek(Number.NaN, 20, true)).toBe(false);
    });
});

describe('formatScrubberPreciseTime', () => {
    test('tenths of a second', () => {
        expect(formatScrubberPreciseTime(0)).toBe('0:00.0');
        expect(formatScrubberPreciseTime(83.47)).toBe('1:23.4');
        expect(formatScrubberPreciseTime(3725.9)).toBe('1:02:05.9');
        expect(formatScrubberPreciseTime(-2)).toBe('0:00.0');
    });
});
