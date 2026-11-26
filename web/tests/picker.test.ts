import { describe, expect, test } from 'bun:test';
import {
    canPick,
    confirmText,
    excludedKeys,
    markerQueueItem,
    MARKER_DEFAULT_PLAY_SECONDS,
    pickedMarkers,
    pickedScenes,
    pickedSummary,
    pickedToQueueItems,
    pickerKey,
    togglePick,
    type PickedItem,
    type PickerOptions,
} from '../app/utils/picker';
import type { SceneListItem } from '../app/types/scene';
import type { MarkerWithScene } from '../app/types/marker';

const scene = (id: number, extra: Partial<SceneListItem> = {}): PickedItem => ({
    kind: 'scene',
    scene: { id, title: `Scene ${id}`, thumbnail_path: 't.jpg', ...extra } as SceneListItem,
});

const marker = (id: number, extra: Partial<MarkerWithScene> = {}): PickedItem => ({
    kind: 'marker',
    marker: {
        id,
        scene_id: 100 + id,
        timestamp: 10,
        end_timestamp: null,
        label: 'Kiss',
        scene_title: 'Video',
        updated_at: '2026-11-26T10:00:00Z',
        ...extra,
    } as MarkerWithScene,
});

const multi: PickerOptions = { multiple: true };
const keys = (items: PickedItem[]) => items.map(pickerKey);

describe('pickerKey and exclusion', () => {
    test('keys tell scenes and markers apart', () => {
        expect(pickerKey(scene(5))).toBe('scene:5');
        expect(pickerKey(marker(5))).toBe('marker:5');
    });

    test('excludedKeys maps both kinds', () => {
        expect([...excludedKeys({ scenes: [1, 2], markers: [2] })]).toEqual([
            'scene:1',
            'scene:2',
            'marker:2',
        ]);
        expect(excludedKeys(undefined).size).toBe(0);
    });
});

describe('togglePick', () => {
    test('multi-select adds in order and removes a selected item', () => {
        let sel: PickedItem[] = [];
        sel = togglePick(sel, scene(1), multi);
        sel = togglePick(sel, marker(1), multi);
        sel = togglePick(sel, scene(2), multi);
        expect(keys(sel)).toEqual(['scene:1', 'marker:1', 'scene:2']);
        sel = togglePick(sel, marker(1), multi);
        expect(keys(sel)).toEqual(['scene:1', 'scene:2']);
    });

    test('single select replaces the selection, a second click clears it', () => {
        const single: PickerOptions = { multiple: false, max: 1 };
        let sel = togglePick([], scene(1), single);
        sel = togglePick(sel, scene(2), single);
        expect(keys(sel)).toEqual(['scene:2']);
        expect(togglePick(sel, scene(2), single)).toEqual([]);
    });

    test('the limit stops adding but still allows removing', () => {
        const opts: PickerOptions = { multiple: true, max: 2 };
        let sel = togglePick(togglePick([], scene(1), opts), scene(2), opts);
        const full = sel;
        sel = togglePick(sel, scene(3), opts);
        expect(sel).toBe(full);
        expect(canPick(sel, scene(3), opts)).toBe(false);
        expect(keys(togglePick(sel, scene(1), opts))).toEqual(['scene:2']);
        // 0 = no limit
        expect(canPick(sel, scene(3), { multiple: true, max: 0 })).toBe(true);
    });

    test('excluded items cannot be picked unless excludedSelectable', () => {
        const excluded = excludedKeys({ scenes: [1], markers: [3] });
        const strict: PickerOptions = { multiple: true, excluded };
        expect(togglePick([], scene(1), strict)).toEqual([]);
        expect(togglePick([], marker(3), strict)).toEqual([]);
        expect(keys(togglePick([], marker(1), strict))).toEqual(['marker:1']);
        // Same id, other kind: not excluded
        expect(canPick([], scene(3), strict)).toBe(true);

        const loose: PickerOptions = { ...strict, excludedSelectable: true };
        expect(keys(togglePick([], scene(1), loose))).toEqual(['scene:1']);
        // Single select ignores the limit but not the exclusion
        expect(canPick([], scene(1), { multiple: false, max: 0, excluded })).toBe(false);
    });
});

describe('queue items of picked items', () => {
    test('a range marker keeps its range and label', () => {
        const m = marker(1, { timestamp: 12.5, end_timestamp: 40, label: '  Kiss  ' });
        expect(markerQueueItem((m as { marker: MarkerWithScene }).marker)).toEqual({
            scene_id: 101,
            start: 12.5,
            end: 40,
            label: 'Kiss',
        });
    });

    test('a marker without an end plays the default duration', () => {
        const m = (marker(2, { timestamp: 50, end_timestamp: null }) as { marker: MarkerWithScene })
            .marker;
        expect(markerQueueItem(m)).toEqual({
            scene_id: 102,
            start: 50,
            end: 50 + MARKER_DEFAULT_PLAY_SECONDS,
            label: 'Kiss',
        });
        expect(markerQueueItem(m, 10).end).toBe(60);
    });

    test('an end not after the start is ignored, an empty label is null', () => {
        const m = (
            marker(3, { timestamp: 20, end_timestamp: 20, label: ' ' }) as {
                marker: MarkerWithScene;
            }
        ).marker;
        expect(markerQueueItem(m, 15)).toEqual({ scene_id: 103, start: 20, end: 35, label: null });
    });

    test('scenes are queued whole, in the selection order', () => {
        const items = [scene(7), marker(1, { end_timestamp: 30 }), scene(8)];
        expect(pickedToQueueItems(items)).toEqual([
            { scene_id: 7 },
            { scene_id: 101, start: 10, end: 30, label: 'Kiss' },
            { scene_id: 8 },
        ]);
        expect(pickedScenes(items).map((s) => s.id)).toEqual([7, 8]);
        expect(pickedMarkers(items).map((m) => m.id)).toEqual([1]);
    });
});

describe('display helpers', () => {
    test('summary of a scene and a marker (versioned marker thumbnail)', () => {
        expect(pickedSummary(scene(4))).toEqual({
            title: 'Scene 4',
            thumb: '/thumbnails/4?size=sm',
        });
        expect(pickedSummary(scene(4, { thumbnail_path: '' })).thumb).toBeNull();
        const s = pickedSummary(marker(9));
        expect(s.title).toBe('Kiss - Video');
        expect(s.thumb).toBe(`/marker-thumbnails/9?v=${Date.parse('2026-11-26T10:00:00Z')}`);
    });

    test('confirm text shows the count in multi-select only', () => {
        expect(confirmText('Add', 3, true)).toBe('Add 3');
        expect(confirmText('Add', 0, true)).toBe('Add');
        expect(confirmText('Replace', 1, false)).toBe('Replace');
    });
});
