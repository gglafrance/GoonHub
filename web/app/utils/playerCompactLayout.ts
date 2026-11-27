// Layout of the scene player controls (composables/usePlayerCompactLayout.ts). On a narrow
// player the single control bar does not fit (with the A/B loop it is about twice the width of
// a phone): on phones and touch screens the controls are spread over the player (compact); on
// desktop the bar stays one bar, tightened (narrow), e.g. for a portrait video. Tests in
// web/tests/playerCompactLayout.test.ts.

/** Below this player width (px) the controls no longer fit the full bar. */
export const COMPACT_PLAYER_WIDTH = 640;

/**
 * Devices that get the spread layout: touch screens, and small viewports (below md, the
 * mobile navigation). A desktop with a narrow player (portrait video) keeps the bar.
 */
export const COMPACT_DEVICE_QUERY = '(pointer: coarse), (max-width: 767.98px)';

/** full: the single bar; narrow: the single bar, tightened; compact: spread over the player. */
export type PlayerControlsMode = 'full' | 'narrow' | 'compact';

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

/** Whether a player of this width is too narrow for the full bar. */
export function isCompactPlayer(width: number): boolean {
    return width > 0 && width < COMPACT_PLAYER_WIDTH;
}

/** Layout of the controls for a player width, on a compact device (touch or small) or not. */
export function playerControlsMode(width: number, compactDevice: boolean): PlayerControlsMode {
    if (!isCompactPlayer(width)) return 'full';
    return compactDevice ? 'compact' : 'narrow';
}
