// Pure helpers of the marker time editing (Markers tab of the watch page: typed start and end,
// nudges). They mirror the API validation of MarkerService.UpdateMarker so that a refused time
// is explained before the request. Unit tested in web/tests/markerTime.test.ts.

/** Marker times typed in the editor: seconds, minutes:seconds or hours:minutes:seconds. */
export const MARKER_TIME_FORMAT_HINT = 'Use ss, mm:ss or h:mm:ss';

export type MarkerTimeField = 'start' | 'end';

/**
 * Seconds of a typed marker time: "75" (seconds), "1:15" (mm:ss, minutes may pass 59) or
 * "1:02:03" (h:mm:ss). Spaces around the parts are ignored. Null when the text is not a time
 * (empty, letters, decimals, negative, seconds or minutes after the first part over 59).
 */
export function parseMarkerTime(text: string): number | null {
    const trimmed = text.trim();
    if (!trimmed) return null;
    const parts = trimmed.split(':').map((p) => p.trim());
    if (parts.length > 3 || parts.some((p) => !/^\d+$/.test(p))) return null;
    const nums = parts.map((p) => parseInt(p, 10));
    // Every part after the first is a two-digit clock part
    if (nums.slice(1).some((n) => n > 59)) return null;
    if (parts.slice(1).some((p) => p.length > 2)) return null;
    return nums.reduce((total, n) => total * 60 + n, 0);
}

/** Marker time as shown on the markers: "mm:ss", or "h:mm:ss" past an hour. */
export function formatMarkerTime(seconds: number): string {
    const total = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds) : 0;
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    const pad = (n: number) => n.toString().padStart(2, '0');
    return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

/** What the validation needs of a marker. */
export interface MarkerTimeRange {
    timestamp: number;
    end_timestamp?: number | null;
}

const knownDuration = (duration?: number | null): number | null =>
    duration != null && duration > 0 ? Math.floor(duration) : null;

/**
 * Why `start` can't be the marker start (null when it can), like the API: not negative, within
 * the scene, and before the end when the marker has one.
 */
export function validateMarkerStart(
    start: number,
    end?: number | null,
    duration?: number | null,
): string | null {
    if (!Number.isFinite(start) || start < 0) return 'The start cannot be negative';
    const max = knownDuration(duration);
    if (max != null && start > max) {
        return `The start is past the end of the scene (${formatMarkerTime(max)})`;
    }
    if (end != null && start >= end) {
        return `The start must be before the end (${formatMarkerTime(end)})`;
    }
    return null;
}

/**
 * Why `end` can't be the marker end (null when it can), like the API: after the start and
 * within the scene.
 */
export function validateMarkerEnd(
    start: number,
    end: number,
    duration?: number | null,
): string | null {
    if (!Number.isFinite(end) || end <= start) {
        return `The end must be after the start (${formatMarkerTime(start)})`;
    }
    const max = knownDuration(duration);
    if (max != null && end > max) {
        return `The end is past the end of the scene (${formatMarkerTime(max)})`;
    }
    return null;
}

/** Why a typed time can't be saved for this field of the marker (null when it can). */
export function validateMarkerTimeEdit(
    field: MarkerTimeField,
    value: number | null,
    marker: MarkerTimeRange,
    duration?: number | null,
): string | null {
    if (value == null) return MARKER_TIME_FORMAT_HINT;
    return field === 'start'
        ? validateMarkerStart(value, marker.end_timestamp, duration)
        : validateMarkerEnd(marker.timestamp, value, duration);
}

/**
 * Typed time moved by `delta` seconds (nudge buttons, ArrowUp/ArrowDown), kept between 0 and the
 * scene duration when it is known. An unreadable text starts from `fallback`.
 */
export function nudgeMarkerTime(
    text: string,
    delta: number,
    fallback: number,
    duration?: number | null,
): string {
    const current = parseMarkerTime(text) ?? fallback;
    const max = knownDuration(duration);
    let next = Math.max(0, current + delta);
    if (max != null) next = Math.min(next, max);
    return formatMarkerTime(next);
}
