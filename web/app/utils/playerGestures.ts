// Pure helpers of the player touch gestures (see usePlayerGestures). They take plain numbers so
// that they can be unit tested without a DOM (web/tests/playerGestures.test.ts).

/** Seek steps of a double-tap on the player sides, in seconds (Settings > Player). */
export const GESTURE_SEEK_STEPS = [5, 10, 15, 30] as const;
export const DEFAULT_GESTURE_SEEK_STEP = 10;

/** Movement (px) under which a touch is still a tap or a long-press. */
export const GESTURE_MOVE_THRESHOLD = 10;
/** Longest touch (ms) that still counts as a tap. */
export const GESTURE_TAP_MAX_MS = 250;
/** Longest gap (ms) between two taps of a double-tap. */
export const GESTURE_DOUBLE_TAP_MS = 300;
/** After a double-tap seek, further single taps on the same side keep seeking for this long (ms). */
export const GESTURE_SEEK_STREAK_MS = 800;
/** Hold (ms) after which a still touch becomes a long-press (temporary 2x speed). */
export const GESTURE_LONG_PRESS_MS = 500;
export const GESTURE_LONG_PRESS_RATE = 2;
/** Seconds covered by a horizontal swipe across the whole player (capped at the duration). */
export const GESTURE_SCRUB_FULL_WIDTH_SECONDS = 120;
export const GESTURE_BRIGHTNESS_MIN = 0.2;
export const GESTURE_BRIGHTNESS_MAX = 1.5;

export type GestureTapZone = 'left' | 'center' | 'right';
export type GestureSide = 'left' | 'right';
export type GestureAxis = 'x' | 'y';

export interface GestureSeekStreak {
    side: GestureSide;
    /** Accumulated seek in seconds, negative when seeking back. */
    amount: number;
    /** Time (ms) of the last tap of the streak. */
    at: number;
}

/** Returns the step when it is one of the allowed values, the default otherwise. */
export function normalizeGestureSeekStep(step: unknown): number {
    return typeof step === 'number' && (GESTURE_SEEK_STEPS as readonly number[]).includes(step)
        ? step
        : DEFAULT_GESTURE_SEEK_STEP;
}

/** Which third of the player a point is in (x relative to the player's left edge). */
export function gestureTapZone(x: number, width: number): GestureTapZone {
    if (width <= 0) return 'center';
    const ratio = x / width;
    if (ratio < 1 / 3) return 'left';
    if (ratio > 2 / 3) return 'right';
    return 'center';
}

/** Which half of the player a point is in. */
export function gestureSide(x: number, width: number): GestureSide {
    return width > 0 && x >= width / 2 ? 'right' : 'left';
}

/** Axis of a drag once it has moved past the threshold, null while it has not. */
export function gestureLockAxis(
    dx: number,
    dy: number,
    threshold = GESTURE_MOVE_THRESHOLD,
): GestureAxis | null {
    const ax = Math.abs(dx);
    const ay = Math.abs(dy);
    if (Math.max(ax, ay) < threshold) return null;
    return ax >= ay ? 'x' : 'y';
}

/** True for a short touch that did not move. */
export function gestureIsTap(dx: number, dy: number, durationMs: number): boolean {
    return (
        Math.abs(dx) < GESTURE_MOVE_THRESHOLD &&
        Math.abs(dy) < GESTURE_MOVE_THRESHOLD &&
        durationMs <= GESTURE_TAP_MAX_MS
    );
}

/**
 * Decides whether a tap seeks, and by how much in total. A tap seeks when it is the second tap
 * of a double-tap on a side third, or when it continues a seek streak (a recent double-tap seek):
 * on the same side it adds a step, on the other side it starts a new streak. A tap in the middle,
 * or a lone tap, returns null (it keeps its normal behaviour: toggling the controls).
 */
export function gestureSeekForTap(
    zone: GestureTapZone,
    now: number,
    lastTapAt: number | null,
    lastTapZone: GestureTapZone | null,
    streak: GestureSeekStreak | null,
    step: number,
): GestureSeekStreak | null {
    if (zone === 'center') return null;
    const delta = zone === 'right' ? step : -step;
    if (streak && now - streak.at <= GESTURE_SEEK_STREAK_MS) {
        if (streak.side === zone) return { side: zone, amount: streak.amount + delta, at: now };
        return { side: zone, amount: delta, at: now };
    }
    if (lastTapAt !== null && lastTapZone === zone && now - lastTapAt <= GESTURE_DOUBLE_TAP_MS) {
        return { side: zone, amount: delta, at: now };
    }
    return null;
}

/** Clamps a playback position to [0, duration] (no upper bound when the duration is unknown). */
export function gestureClampTime(time: number, duration: number): number {
    const t = Math.max(0, time);
    return duration > 0 ? Math.min(duration, t) : t;
}

/** Target time of a horizontal scrub of dx pixels on a player of the given width. */
export function gestureScrubTime(
    startTime: number,
    dx: number,
    width: number,
    duration: number,
): number {
    if (width <= 0) return gestureClampTime(startTime, duration);
    const span = duration > 0 ? Math.min(duration, GESTURE_SCRUB_FULL_WIDTH_SECONDS) : 0;
    return gestureClampTime(startTime + (dx / width) * span, duration);
}

/**
 * Level (volume, brightness) after a vertical swipe of dy pixels: swiping up raises it, and a
 * swipe over the whole player height covers the whole [min, max] range.
 */
export function gestureSwipeLevel(
    start: number,
    dy: number,
    height: number,
    min: number,
    max: number,
): number {
    if (height <= 0) return Math.min(max, Math.max(min, start));
    const value = start - (dy / height) * (max - min);
    return Math.min(max, Math.max(min, value));
}

/** "+30s" / "-10s" for an accumulated seek. */
export function formatGestureSeek(amount: number): string {
    return `${amount >= 0 ? '+' : '-'}${Math.abs(Math.round(amount))}s`;
}

/** Clock time: "1:05", "12:00", "1:02:03". */
export function formatGestureClock(seconds: number): string {
    const total = Math.max(0, Math.floor(seconds));
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    const ss = s.toString().padStart(2, '0');
    return h > 0 ? `${h}:${m.toString().padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}

/** Signed clock offset of a scrub: "+1:05", "-0:12". */
export function formatGestureOffset(delta: number): string {
    return `${delta < 0 ? '-' : '+'}${formatGestureClock(Math.abs(delta))}`;
}

/** Sprite cue covering a time, if any (cues from the scene's sprite VTT). */
export function findGestureCue<T extends { start: number; end: number }>(
    cues: readonly T[],
    time: number,
): T | null {
    return cues.find((c) => time >= c.start && time < c.end) ?? null;
}
