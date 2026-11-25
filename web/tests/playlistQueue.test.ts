import { describe, expect, test } from 'bun:test';
import {
    crossedRangeEnd,
    markerPlaylistFilter,
    playlistItemQuery,
    playlistQueueItems,
    playlistResumeTarget,
} from '../app/utils/playlistQueue';
import type { PlaylistDetail, PlaylistMarkerEntry } from '../app/types/playlist';

const markerEntry = (
    id: number,
    sceneId: number,
    start: number,
    end: number,
): PlaylistMarkerEntry => ({
    position: 0,
    added_at: '2026-11-25T10:00:00Z',
    marker: {
        id,
        user_id: 1,
        scene_id: sceneId,
        timestamp: start,
        end_timestamp: null,
        label: id === 2 ? '' : `Label ${id}`,
        color: '#FFFFFF',
        has_thumbnail: id !== 2,
        thumbnail_version: 1700000000123,
    },
    scene: { id: sceneId, title: `Scene ${sceneId}`, duration: 600 },
    start,
    end,
});

type Detail = Pick<PlaylistDetail, 'kind' | 'scenes' | 'markers' | 'resume'>;

const markerDetail = (resume: Detail['resume'] = null): Detail => ({
    kind: 'markers',
    scenes: [],
    markers: [markerEntry(1, 10, 40, 70), markerEntry(2, 10, 100, 130), markerEntry(3, 11, 5, 35)],
    resume,
});

describe('playlistQueueItems', () => {
    test('a marker playlist plays the marker ranges', () => {
        const items = playlistQueueItems(markerDetail());
        expect(items.map((i) => i.key)).toEqual(['marker-1', 'marker-2', 'marker-3']);
        expect(items[0]).toMatchObject({
            sceneId: 10,
            markerId: 1,
            title: 'Label 1',
            subtitle: 'Scene 10',
            start: 40,
            end: 70,
            duration: 30,
            thumbnailUrl: '/marker-thumbnails/1?v=1700000000123',
        });
        // No label, no thumbnail
        expect(items[1]!.title).toBe('Marker');
        expect(items[1]!.thumbnailUrl).toBeNull();
    });

    test('a scene playlist plays whole scenes', () => {
        const items = playlistQueueItems({
            kind: 'scenes',
            markers: [],
            scenes: [
                {
                    position: 0,
                    added_at: '',
                    scene: { id: 7, title: 'Seven', duration: 120, thumbnail_path: 'x' },
                },
            ] as unknown as PlaylistDetail['scenes'],
        });
        expect(items).toHaveLength(1);
        expect(items[0]).toMatchObject({
            key: 'scene-7',
            sceneId: 7,
            markerId: null,
            start: null,
            end: null,
            duration: 120,
            thumbnailUrl: '/thumbnails/7',
        });
    });
});

describe('playlistItemQuery', () => {
    test('a marker starts at its time, without an end (no loop)', () => {
        const [item] = playlistQueueItems(markerDetail());
        expect(playlistItemQuery('u', item!, 2, 99)).toEqual({
            playlist: 'u',
            pos: '2',
            shuffle: '99',
            t: '40',
        });
        expect(playlistItemQuery('u', item!, 0, 0, 52.7)).toEqual({
            playlist: 'u',
            pos: '0',
            t: '52',
        });
    });

    test('a scene starts from the beginning unless resumed', () => {
        expect(playlistItemQuery('u', { start: null }, 1)).toEqual({ playlist: 'u', pos: '1' });
        expect(playlistItemQuery('u', { start: null }, 1, 0, 90)).toEqual({
            playlist: 'u',
            pos: '1',
            t: '90',
        });
    });
});

describe('crossedRangeEnd', () => {
    test('ends when playback reaches the end', () => {
        expect(crossedRangeEnd(70, 69.8, 70.1)).toBe(true);
        expect(crossedRangeEnd(70, 69, 69.9)).toBe(false);
    });

    test('a seek past the end does not end the marker', () => {
        expect(crossedRangeEnd(70, 50, 200)).toBe(false);
        // Already past the end (kept watching)
        expect(crossedRangeEnd(70, 75, 75.3)).toBe(false);
    });
});

describe('playlistResumeTarget', () => {
    test('resumes a marker inside its range, else at its start', () => {
        const inside = playlistResumeTarget(
            markerDetail({ scene_id: 10, marker_id: 2, position_s: 110 }),
        );
        expect(inside).toMatchObject({ index: 1, startAt: 110 });
        const outside = playlistResumeTarget(
            markerDetail({ scene_id: 10, marker_id: 2, position_s: 300 }),
        );
        expect(outside).toMatchObject({ index: 1, startAt: 100 });
    });

    test('nothing to resume', () => {
        expect(playlistResumeTarget(markerDetail(null))).toBeNull();
        expect(
            playlistResumeTarget(markerDetail({ scene_id: 10, marker_id: 99, position_s: 1 })),
        ).toBeNull();
    });
});

describe('markerPlaylistFilter', () => {
    test('maps the listing query parameters', () => {
        expect(
            markerPlaylistFilter(
                { labels: 'A,B', ranges_only: 'true', seed: '42', tags: 't', studio: 'S' },
                { sort: 'random' },
            ),
        ).toEqual({
            labels: 'A,B',
            ranges_only: true,
            seed: 42,
            tags: 't',
            studio: 'S',
            sort: 'random',
        });
        expect(markerPlaylistFilter({}, { label: 'Kiss' })).toEqual({ label: 'Kiss' });
    });
});
