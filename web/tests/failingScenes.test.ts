import { describe, expect, test } from 'bun:test';
import {
    errorPreview,
    failingActionMessage,
    failingBulkBody,
    failingPhaseLabel,
    failingPhasesWithTotals,
    failingQueryString,
    failingTotal,
    isErrorTruncated,
    toggleId,
    togglePage,
} from '../app/utils/failingScenes';

describe('failingQueryString', () => {
    test('leaves empty filters out and trims the search', () => {
        expect(failingQueryString({ page: 1, limit: 25 })).toBe('page=1&limit=25');
        expect(failingQueryString({ phase: 'sprites', q: '  parse duration ', page: 3 })).toBe(
            'phase=sprites&q=parse+duration&page=3&limit=25',
        );
        expect(failingQueryString({ q: '   ' })).toBe('page=1&limit=25');
    });
    test('bounds page and limit', () => {
        expect(failingQueryString({ page: 0, limit: -4 })).toBe('page=1&limit=1');
    });
});

describe('errorPreview', () => {
    test('keeps a short error', () => {
        expect(errorPreview('video file is corrupted')).toBe('video file is corrupted');
        expect(isErrorTruncated('video file is corrupted')).toBe(false);
    });
    test('cuts a long error and keeps the first line only', () => {
        const long = 'ffmpeg failed extracting frame at 00:12:34: ' + 'x'.repeat(200);
        const preview = errorPreview(long, 40);
        expect(preview.length).toBe(40);
        expect(preview.endsWith('…')).toBe(true);
        expect(isErrorTruncated(long, 40)).toBe(true);
        expect(errorPreview('first line\nstack trace')).toBe('first line');
        expect(isErrorTruncated('first line\nstack trace')).toBe(true);
    });
    test('empty error', () => {
        expect(errorPreview('')).toBe('');
    });
});

describe('phases and totals', () => {
    test('labels', () => {
        expect(failingPhaseLabel('animated_thumbnails')).toBe('Animated thumbnails');
        expect(failingPhaseLabel('custom')).toBe('custom');
    });
    test('total and phases with failing scenes, known order first', () => {
        const totals = { sprites: 153, metadata: 341, thumbnail: 0, zzz: 2 };
        expect(failingTotal(totals)).toBe(496);
        expect(failingPhasesWithTotals(totals)).toEqual(['metadata', 'sprites', 'zzz']);
        expect(failingPhasesWithTotals({})).toEqual([]);
    });
});

describe('selection', () => {
    test('toggle one row', () => {
        expect(toggleId([1, 2], 3)).toEqual([1, 2, 3]);
        expect(toggleId([1, 2, 3], 2)).toEqual([1, 3]);
    });
    test('toggle the page', () => {
        expect(togglePage([9], [1, 2])).toEqual([9, 1, 2]);
        expect(togglePage([9, 1], [1, 2])).toEqual([9, 1, 2]);
        expect(togglePage([9, 1, 2], [1, 2])).toEqual([9]);
        expect(togglePage([9], [])).toEqual([9]);
    });
});

describe('failingBulkBody', () => {
    test('selected IDs win over the filters', () => {
        expect(failingBulkBody('retry', [4, 5], { phase: 'sprites', q: 'x' })).toEqual({
            action: 'retry',
            ids: [4, 5],
        });
    });
    test('no selection: every match of the filters', () => {
        expect(failingBulkBody('clear', [], { phase: 'metadata', q: ' corrupted ' })).toEqual({
            action: 'clear',
            all: true,
            phase: 'metadata',
            q: 'corrupted',
        });
        expect(failingBulkBody('retry', [], {})).toEqual({ action: 'retry', all: true });
    });
});

describe('failingActionMessage', () => {
    test('retry and clear', () => {
        expect(failingActionMessage('retry', { retried: 1, cleared: 1, errors: 0 })).toBe(
            'Queued 1 job',
        );
        expect(failingActionMessage('retry', { retried: 3, cleared: 4, errors: 1 })).toBe(
            'Queued 3 jobs (1 could not be queued)',
        );
        expect(failingActionMessage('clear', { retried: 0, cleared: 2, errors: 0 })).toBe(
            'Forgot the failures of 2 scenes',
        );
    });
});
