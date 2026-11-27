// Compact layout of the scene player controls (composables/usePlayerCompactLayout.ts): on a
// narrow player the single control bar does not fit (with the A/B loop it is about twice the
// width of a phone), so the controls are spread over the player. Tests in
// web/tests/playerCompactLayout.test.ts.

/** Below this player width (px) the controls use the compact layout. */
export const COMPACT_PLAYER_WIDTH = 640;

export type CompactSlot = 'top' | 'center' | 'bottom';

/** Secondary controls, in a bar at the top of the player (in this order). */
export const COMPACT_TOP_CONTROLS: readonly string[] = [
    'vjs-playback-rate',
    'vjs-ab-loop-start',
    'vjs-ab-loop-end',
    'vjs-ab-loop-toggle',
    'vjs-picture-in-picture-control',
    'vjs-scrubber-toggle-control',
];

/** Playback, in the middle of the player (in this order). */
export const COMPACT_CENTER_CONTROLS: readonly string[] = [
    'vjs-playlist-prev-control',
    'vjs-play-control',
    'vjs-playlist-next-control',
];

/**
 * Where a control bar child goes in the compact layout, from its classes; everything else
 * (progress, mute, time, fullscreen) stays in the bottom bar.
 */
export function compactSlotOf(classList: Iterable<string>): CompactSlot {
    const classes = new Set(classList);
    if (COMPACT_TOP_CONTROLS.some((c) => classes.has(c))) return 'top';
    if (COMPACT_CENTER_CONTROLS.some((c) => classes.has(c))) return 'center';
    return 'bottom';
}

/** Order of a control inside its compact slot (unknown controls last, stable). */
export function compactOrderOf(classList: Iterable<string>): number {
    const classes = [...classList];
    for (const list of [COMPACT_TOP_CONTROLS, COMPACT_CENTER_CONTROLS]) {
        const i = list.findIndex((c) => classes.includes(c));
        if (i >= 0) return i;
    }
    return Number.MAX_SAFE_INTEGER;
}

/** Whether a player of this width uses the compact layout. */
export function isCompactPlayer(width: number): boolean {
    return width > 0 && width < COMPACT_PLAYER_WIDTH;
}
