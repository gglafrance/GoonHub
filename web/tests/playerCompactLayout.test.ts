import { describe, expect, test } from 'bun:test';
import {
    COMPACT_PLAYER_WIDTH,
    compactOrderOf,
    compactSlotOf,
    isCompactPlayer,
    playerControlsMode,
} from '../app/utils/playerCompactLayout';

const cls = (s: string) => s.split(' ');

describe('player compact layout', () => {
    test('slots', () => {
        expect(compactSlotOf(cls('vjs-playback-rate vjs-menu-button vjs-control'))).toBe('top');
        expect(compactSlotOf(cls('vjs-ab-loop-time vjs-ab-loop-start vjs-control'))).toBe('top');
        expect(compactSlotOf(cls('vjs-ab-loop-toggle vjs-control'))).toBe('top');
        expect(compactSlotOf(cls('vjs-picture-in-picture-control vjs-control'))).toBe('top');
        expect(compactSlotOf(cls('vjs-scrubber-toggle-control vjs-control'))).toBe('top');
        expect(compactSlotOf(cls('vjs-play-control vjs-control vjs-paused'))).toBe('center');
        expect(compactSlotOf(cls('vjs-playlist-next-control vjs-control'))).toBe('center');
        for (const c of [
            'vjs-progress-control vjs-control',
            'vjs-volume-panel vjs-control',
            'vjs-current-time vjs-time-control',
            'vjs-fullscreen-control vjs-control',
            'vjs-theater-mode-control vjs-control',
        ]) {
            expect(compactSlotOf(cls(c))).toBe('bottom');
        }
    });

    test('order inside a slot', () => {
        const center = [
            'vjs-play-control',
            'vjs-playlist-next-control',
            'vjs-playlist-prev-control',
        ].sort((a, b) => compactOrderOf([a]) - compactOrderOf([b]));
        expect(center).toEqual([
            'vjs-playlist-prev-control',
            'vjs-play-control',
            'vjs-playlist-next-control',
        ]);
        expect(compactOrderOf(['unknown'])).toBe(Number.MAX_SAFE_INTEGER);
    });

    test('width threshold', () => {
        expect(isCompactPlayer(0)).toBe(false);
        expect(isCompactPlayer(360)).toBe(true);
        expect(isCompactPlayer(COMPACT_PLAYER_WIDTH - 1)).toBe(true);
        expect(isCompactPlayer(COMPACT_PLAYER_WIDTH)).toBe(false);
    });

    test('spread layout on phones and touch screens only', () => {
        // a portrait video on desktop: narrow player, the bar stays
        expect(playerControlsMode(480, false)).toBe('narrow');
        expect(playerControlsMode(480, true)).toBe('compact');
        expect(playerControlsMode(900, true)).toBe('full');
        expect(playerControlsMode(900, false)).toBe('full');
        expect(playerControlsMode(0, true)).toBe('full');
    });
});
