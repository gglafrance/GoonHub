import { describe, expect, test } from 'bun:test';
import {
    markerThumbnailUrl,
    markerThumbnailVersion,
    markerThumbnailVersionChanged,
} from '../app/utils/markerThumbnail';

describe('markerThumbnailVersion', () => {
    test('keeps Unix milliseconds from the API', () => {
        expect(markerThumbnailVersion(1700000000123)).toBe(1700000000123);
        expect(markerThumbnailVersion(1700000000123.9)).toBe(1700000000123);
    });

    test('reads updated_at dates, dropping digits past the millisecond like Go', () => {
        expect(markerThumbnailVersion('2023-11-14T22:13:20.123456789Z')).toBe(1700000000123);
        expect(markerThumbnailVersion('2023-11-14T22:13:20.123999Z')).toBe(1700000000123);
        expect(markerThumbnailVersion('2023-11-14T23:13:20.123456+01:00')).toBe(1700000000123);
        expect(markerThumbnailVersion('2023-11-14T22:13:20Z')).toBe(1700000000000);
        expect(markerThumbnailVersion('2023-11-14T22:13:20.5Z')).toBe(1700000000500);
    });

    test('is null when unknown or invalid', () => {
        expect(markerThumbnailVersion(undefined)).toBeNull();
        expect(markerThumbnailVersion(null)).toBeNull();
        expect(markerThumbnailVersion('')).toBeNull();
        expect(markerThumbnailVersion('not a date')).toBeNull();
        expect(markerThumbnailVersion('0001-01-01T00:00:00Z')).toBeNull();
        expect(markerThumbnailVersion(0)).toBeNull();
        expect(markerThumbnailVersion(Number.NaN)).toBeNull();
    });
});

describe('markerThumbnailUrl', () => {
    test('versions the static and animated URLs', () => {
        expect(markerThumbnailUrl(12, 1700000000123)).toBe('/marker-thumbnails/12?v=1700000000123');
        expect(markerThumbnailUrl(12, '2023-11-14T22:13:20.123Z', true)).toBe(
            '/marker-thumbnails/12/animated?v=1700000000123',
        );
    });

    test('falls back to the plain URL without a version', () => {
        expect(markerThumbnailUrl(12)).toBe('/marker-thumbnails/12');
        expect(markerThumbnailUrl(12, null, true)).toBe('/marker-thumbnails/12/animated');
    });

    test('a new updated_at gives a new URL', () => {
        const before = markerThumbnailUrl(3, '2026-11-22T10:00:00.100Z');
        const after = markerThumbnailUrl(3, '2026-11-22T10:00:04.250Z');
        expect(after).not.toBe(before);
    });
});

describe('markerThumbnailVersionChanged', () => {
    test('detects a thumbnail written after the update', () => {
        expect(
            markerThumbnailVersionChanged('2026-11-22T10:00:00.100Z', '2026-11-22T10:00:04.250Z'),
        ).toBe(true);
    });

    test('ignores the same version and database rounding', () => {
        expect(
            markerThumbnailVersionChanged(
                '2026-11-22T10:00:00.100999999Z',
                '2026-11-22T10:00:00.101Z',
            ),
        ).toBe(false);
        expect(markerThumbnailVersionChanged(1000, 1000)).toBe(false);
    });

    test('unknown versions', () => {
        expect(markerThumbnailVersionChanged(null, 1000)).toBe(true);
        expect(markerThumbnailVersionChanged(1000, null)).toBe(false);
    });
});
