// Pure helpers of the thumbnail scrubber strip under the player (see components/watch/Scrubber.vue
// and composables/useScrubberScroll.ts). They take plain numbers so that they can be unit tested
// without a DOM (web/tests/scrubber.test.ts).

import type { ScrubberMode } from '~/types/settings';

// Height of a thumbnail of the strip, in CSS pixels (the width follows the sprite aspect ratio)
export const SCRUBBER_TILE_HEIGHT = 64;
// Thumbnails rendered on each side of the visible ones
export const SCRUBBER_OVERSCAN = 4;
// Following playback resumes this long after the user stopped scrolling the strip
export const SCRUBBER_FOLLOW_RESUME_MS = 4000;
// While following, the strip scrolls when the position leaves [lead, trail] of the visible width,
// and brings it back to lead
export const SCRUBBER_FOLLOW_LEAD = 0.25;
export const SCRUBBER_FOLLOW_TRAIL = 0.8;
// A press that moves less than this is a click, not a drag (px)
export const SCRUBBER_DRAG_THRESHOLD = 5;
// Inertia after a drag: velocity kept per 16 ms frame, and velocity under which it stops (px/ms)
export const SCRUBBER_FRICTION = 0.94;
export const SCRUBBER_MIN_VELOCITY = 0.02;
// Page buttons scroll this fraction of the visible width
export const SCRUBBER_PAGE_FRACTION = 0.8;

// Time range of a thumbnail (a VTT cue)
export interface ScrubberCue {
    start: number;
    end: number;
}

const clamp = (v: number, min: number, max: number) => Math.min(max, Math.max(min, v));

// Width of a thumbnail for a sprite tile of w x h pixels shown `height` pixels high.
export function scrubberTileWidth(w: number, h: number, height = SCRUBBER_TILE_HEIGHT): number {
    if (!(w > 0) || !(h > 0)) return Math.round((height * 16) / 9);
    return Math.max(16, Math.round((height * w) / h));
}

// Index of the cue shown at `time`: the last cue that starts at or before it (cues are sorted).
// Before the first cue: 0. No cues: -1.
export function findCueIndex(cues: readonly ScrubberCue[], time: number): number {
    if (cues.length === 0) return -1;
    if (!Number.isFinite(time) || time <= cues[0]!.start) return 0;
    let lo = 0;
    let hi = cues.length - 1;
    while (lo < hi) {
        const mid = (lo + hi + 1) >> 1;
        if (cues[mid]!.start <= time) lo = mid;
        else hi = mid - 1;
    }
    return lo;
}

// Horizontal position in the strip content of `time` (px from the start of the first thumbnail).
export function timeToOffset(
    cues: readonly ScrubberCue[],
    time: number,
    tileWidth: number,
): number {
    const i = findCueIndex(cues, time);
    if (i < 0) return 0;
    const cue = cues[i]!;
    const span = cue.end - cue.start;
    const frac = span > 0 ? clamp((time - cue.start) / span, 0, 1) : 0;
    return (i + frac) * tileWidth;
}

// Scene time at a horizontal position in the strip content (inverse of timeToOffset).
export function offsetToTime(
    cues: readonly ScrubberCue[],
    offset: number,
    tileWidth: number,
): number {
    if (cues.length === 0 || !(tileWidth > 0)) return 0;
    const pos = clamp(offset / tileWidth, 0, cues.length);
    const i = Math.min(cues.length - 1, Math.floor(pos));
    const cue = cues[i]!;
    return cue.start + (pos - i) * Math.max(0, cue.end - cue.start);
}

// Index of the thumbnail under a horizontal position in the strip content (-1 outside).
export function tileIndexAt(offset: number, tileWidth: number, count: number): number {
    if (!(tileWidth > 0) || offset < 0) return -1;
    const i = Math.floor(offset / tileWidth);
    return i < count ? i : -1;
}

// Thumbnails to render for the visible part of the strip, overscan included (inclusive range;
// last < first when there is nothing to render).
export function visibleTileRange(
    scrollLeft: number,
    viewWidth: number,
    tileWidth: number,
    count: number,
    overscan = SCRUBBER_OVERSCAN,
): { first: number; last: number } {
    if (count <= 0 || !(tileWidth > 0)) return { first: 0, last: -1 };
    const first = clamp(Math.floor(scrollLeft / tileWidth) - overscan, 0, count - 1);
    const last = clamp(
        Math.ceil((scrollLeft + Math.max(0, viewWidth)) / tileWidth) - 1 + overscan,
        first,
        count - 1,
    );
    return { first, last };
}

export function maxScroll(viewWidth: number, contentWidth: number): number {
    return Math.max(0, contentWidth - viewWidth);
}

export function clampScroll(left: number, viewWidth: number, contentWidth: number): number {
    return clamp(left, 0, maxScroll(viewWidth, contentWidth));
}

// Scroll position that keeps the playback position in view while following, or null when it
// is already inside the comfortable zone [lead, trail] of the visible width.
export function followScrollTarget(
    positionX: number,
    scrollLeft: number,
    viewWidth: number,
    contentWidth: number,
    lead = SCRUBBER_FOLLOW_LEAD,
    trail = SCRUBBER_FOLLOW_TRAIL,
): number | null {
    if (!(viewWidth > 0)) return null;
    const rel = positionX - scrollLeft;
    if (rel >= viewWidth * lead && rel <= viewWidth * trail) return null;
    const target = clampScroll(positionX - viewWidth * lead, viewWidth, contentWidth);
    return Math.abs(target - scrollLeft) < 1 ? null : target;
}

// Whether following playback resumes after the user scrolled the strip: only while playing
// (a paused strip stays where the user left it), and once the user has been idle long enough.
export function shouldResumeFollow(
    userIdleMs: number,
    playing: boolean,
    interacting: boolean,
    resumeMs = SCRUBBER_FOLLOW_RESUME_MS,
): boolean {
    return playing && !interacting && userIdleMs >= resumeMs;
}

// Target of a page button: most of a visible width to the left (-1) or right (1).
export function pageScrollTarget(
    scrollLeft: number,
    viewWidth: number,
    contentWidth: number,
    direction: -1 | 1,
    fraction = SCRUBBER_PAGE_FRACTION,
): number {
    return clampScroll(scrollLeft + direction * viewWidth * fraction, viewWidth, contentWidth);
}

// Horizontal scroll of a wheel event, in pixels: the dominant axis (a vertical wheel scrolls
// the strip sideways), lines and pages converted to pixels.
export function wheelScrollDelta(
    deltaX: number,
    deltaY: number,
    deltaMode: number,
    pageWidth: number,
): number {
    const delta = Math.abs(deltaX) > Math.abs(deltaY) ? deltaX : deltaY;
    if (deltaMode === 1) return delta * 40;
    if (deltaMode === 2) return delta * pageWidth;
    return delta;
}

export interface DragSample {
    x: number;
    t: number;
}

// Velocity of a drag at its release, in px/ms of content scroll (opposite of the pointer
// movement), from the samples of the last `windowMs` milliseconds.
export function releaseVelocity(samples: readonly DragSample[], windowMs = 100): number {
    if (samples.length < 2) return 0;
    const last = samples[samples.length - 1]!;
    let first = last;
    for (let i = samples.length - 2; i >= 0; i--) {
        const s = samples[i]!;
        if (last.t - s.t > windowMs) break;
        first = s;
    }
    const dt = last.t - first.t;
    if (dt <= 0) return 0;
    return -(last.x - first.x) / dt;
}

// Velocity after `dtMs` milliseconds of inertia (0 once it is too slow to matter).
export function decayVelocity(
    velocity: number,
    dtMs: number,
    friction = SCRUBBER_FRICTION,
    minVelocity = SCRUBBER_MIN_VELOCITY,
): number {
    const v = velocity * Math.pow(friction, Math.max(0, dtMs) / 16);
    return Math.abs(v) < minVelocity ? 0 : v;
}

// One frame of the smooth scroll towards a target: the remaining distance shrinks by
// `factor`, and the last pixel is reached directly.
export function smoothScrollStep(current: number, target: number, factor = 0.22): number {
    const diff = target - current;
    if (Math.abs(diff) < 0.5) return target;
    return current + diff * factor;
}

// Cue of the thumbnail before (-1) or after (1) the one shown at `time` (keyboard navigation).
// A time a hair before a cue start (rounding of a seek) counts as that cue.
export function adjacentCueIndex(
    cues: readonly ScrubberCue[],
    time: number,
    direction: -1 | 1,
): number {
    if (cues.length === 0) return -1;
    const i = findCueIndex(cues, time + 0.05);
    return clamp(i + direction, 0, cues.length - 1);
}

// Clock of a thumbnail: m:ss, or h:mm:ss from one hour.
export function formatScrubberTime(seconds: number): string {
    const total = Math.max(0, Math.floor(Number.isFinite(seconds) ? seconds : 0));
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    const ss = String(s).padStart(2, '0');
    return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}

// Left positions of the marker chips of the strip: each chip starts at its marker, shifted right
// of the previous chip when they would overlap, and kept inside the content (`maxRight`).
export function layoutChips(
    items: readonly { x: number; width: number }[],
    maxRight = Infinity,
    gap = 2,
): number[] {
    const order = items.map((_, i) => i).sort((a, b) => items[a]!.x - items[b]!.x);
    const result: number[] = new Array(items.length).fill(0);
    let lastEnd = -Infinity;
    for (const i of order) {
        const item = items[i]!;
        const x = Math.max(0, Math.min(Math.max(item.x, lastEnd + gap), maxRight - item.width));
        result[i] = x;
        lastEnd = x + item.width;
    }
    return result;
}

// Estimated width of a marker chip with this label (no DOM measuring), in px.
export function markerChipWidth(label: string, maxWidth = 140): number {
    return Math.min(maxWidth, 18 + Math.max(1, label.length) * 5.6);
}

// ---- Centered mode (like Stash): the centre line of the strip is the playback position, and
// moving the strip seeks the video. Offsets are in content pixels, as for timeToOffset.

// Seeks of a drag closer than this to the previous one are skipped (s)
export const SCRUBBER_SEEK_MIN_DELTA = 0.02;
// A seek that has not finished after this long does not hold the next one back (ms)
export const SCRUBBER_SEEK_MAX_WAIT_MS = 250;
// Moving the strip stops this far before the end of the scene, so that a drag does not end it (s)
export const SCRUBBER_END_MARGIN = 0.1;
// A wheel scroll is over once no wheel event came for this long: the precise seek follows (ms)
export const SCRUBBER_WHEEL_IDLE_MS = 180;

export function normalizeScrubberMode(mode: unknown): ScrubberMode {
    return mode === 'centered' ? 'centered' : 'free';
}

// Centre offset clamped to the strip content: from the start of the first thumbnail to the end
// of the last one.
export function clampCenterOffset(offset: number, contentWidth: number): number {
    if (!Number.isFinite(offset)) return 0;
    return clamp(offset, 0, Math.max(0, contentWidth));
}

// Translation of the content that puts `offset` under the centre line of a view `viewWidth` wide.
export function centeredTranslate(offset: number, viewWidth: number): number {
    return Math.max(0, viewWidth) / 2 - offset;
}

// Left edge, in content pixels, of the visible part of the content (negative before the start
// of the strip), for visibleTileRange.
export function centeredViewLeft(offset: number, viewWidth: number): number {
    return offset - Math.max(0, viewWidth) / 2;
}

// Centre offset while dragging: the content follows the pointer (moving it right goes back in
// time), clamped to the content.
export function dragCenterOffset(
    startOffset: number,
    startX: number,
    x: number,
    contentWidth: number,
): number {
    return clampCenterOffset(startOffset - (x - startX), contentWidth);
}

// Scene time under the centre line: the linear time/position mapping of the thumbnails, kept
// inside the scene (0 to a hair before the end; a duration of 0 is unknown: the cues bound it).
export function centerTime(
    cues: readonly ScrubberCue[],
    offset: number,
    tileWidth: number,
    duration: number,
    endMargin = SCRUBBER_END_MARGIN,
): number {
    const time = offsetToTime(cues, clampCenterOffset(offset, cues.length * tileWidth), tileWidth);
    const max = duration > 0 ? Math.max(0, duration - endMargin) : Infinity;
    return clamp(time, 0, max);
}

// Centre offset showing `time` (inverse of centerTime).
export function centerOffsetForTime(
    cues: readonly ScrubberCue[],
    time: number,
    tileWidth: number,
): number {
    return clampCenterOffset(timeToOffset(cues, time, tileWidth), cues.length * tileWidth);
}

// Whether a drag sends a seek to `target` now: not when it is (almost) where the last seek went,
// and not while the previous seek is still running (the frames would pile up), unless that one
// has been running for too long. lastIssued is NaN when nothing was sent yet.
export function shouldIssueSeek(
    target: number,
    lastIssued: number,
    seeking: boolean,
    sinceLastIssueMs: number,
    minDelta = SCRUBBER_SEEK_MIN_DELTA,
    maxWaitMs = SCRUBBER_SEEK_MAX_WAIT_MS,
): boolean {
    if (!Number.isFinite(target)) return false;
    if (Number.isFinite(lastIssued) && Math.abs(target - lastIssued) < minDelta) return false;
    return !seeking || sinceLastIssueMs >= maxWaitMs;
}

// Whether the end of a drag still needs a precise seek: the last seek was a fast (keyframe)
// one, went somewhere else, or there was none.
export function needsFinalSeek(target: number, lastIssued: number, lastWasFast: boolean): boolean {
    if (!Number.isFinite(target)) return false;
    return lastWasFast || !Number.isFinite(lastIssued) || Math.abs(target - lastIssued) > 1e-3;
}

// Clock of the centre line: m:ss.t (tenths), or h:mm:ss.t from one hour.
export function formatScrubberPreciseTime(seconds: number): string {
    const value = Math.max(0, Number.isFinite(seconds) ? seconds : 0);
    const tenths = Math.floor(value * 10) % 10;
    return `${formatScrubberTime(value)}.${tenths}`;
}
