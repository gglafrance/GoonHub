/** A part of a scene that was played continuously: [start, end] in seconds. */
export type WatchedRange = [number, number];

export interface WatchedRangeTracker {
    /** Playback reached `time` (call on timeupdate while playing). */
    advance: (time: number, playbackRate?: number) => void;
    /** Ends the current range (pause, seek, end of the scene). */
    close: () => void;
    /** Returns the ranges played since the last call; the current range goes on after it. */
    take: () => WatchedRange[];
    /** Forgets everything (scene change). */
    reset: () => void;
}

const round = (seconds: number) => Math.round(seconds * 100) / 100;

/**
 * Collects the parts of a scene that are actually played. A range grows while the playback
 * position moves forward by small steps; a seek (reported with close(), or detected as a
 * jump) ends it, so the skipped part is never counted, and replaying a part yields a new
 * range over it.
 */
export function createWatchedRangeTracker(maxStepSeconds = 4): WatchedRangeTracker {
    let current: { start: number; end: number } | null = null;
    let done: WatchedRange[] = [];

    const close = () => {
        if (current && current.end > current.start) {
            done.push([current.start, current.end]);
        }
        current = null;
    };

    const advance = (time: number, playbackRate = 1) => {
        if (!Number.isFinite(time) || time < 0) return;
        if (!current) {
            current = { start: time, end: time };
            return;
        }
        const step = time - current.end;
        if (step >= 0 && step <= maxStepSeconds * Math.max(1, playbackRate)) {
            current.end = time;
            return;
        }
        close();
        current = { start: time, end: time };
    };

    const take = (): WatchedRange[] => {
        const ranges = done;
        done = [];
        if (current && current.end > current.start) {
            ranges.push([current.start, current.end]);
            current = { start: current.end, end: current.end };
        }
        // Rounded the same way on both sides of a split, so consecutive chunks still meet
        return ranges
            .map(([start, end]): WatchedRange => [round(start), round(end)])
            .filter(([start, end]) => end > start);
    };

    const reset = () => {
        current = null;
        done = [];
    };

    return { advance, close, take, reset };
}

export interface ReplayHeatmapPaths {
    /** Filled area under the curve, closed on the baseline. */
    area: string;
    /** The curve alone. */
    line: string;
}

/**
 * Builds a smooth "most replayed" curve from histogram buckets, scaled to the most watched
 * part, in a width x height box (y grows downwards, baseline at the bottom). Returns null
 * when nothing was watched.
 */
export function buildReplayHeatmapPaths(
    buckets: number[],
    width = 1000,
    height = 100,
): ReplayHeatmapPaths | null {
    const n = buckets.length;
    if (n === 0) return null;
    // Light smoothing so that a single bucket reads as a bump rather than a spike
    const values = buckets.map((v, i) => {
        const prev = buckets[i - 1] ?? v;
        const next = buckets[i + 1] ?? v;
        return (prev + 2 * v + next) / 4;
    });
    const max = Math.max(...values);
    if (!(max > 0)) return null;

    const top = height * 0.08;
    const points = values.map((v, i): [number, number] => [
        ((i + 0.5) / n) * width,
        height - (v / max) * (height - top),
    ]);
    const clampY = (y: number) => Math.min(height, Math.max(0, y));
    const fmt = (x: number) => x.toFixed(1);

    // Catmull-Rom spline through the points, as cubic Bezier segments
    const first = points[0]!;
    const last = points[n - 1]!;
    let curve = `L${fmt(first[0])},${fmt(first[1])}`;
    for (let i = 0; i < n - 1; i++) {
        const p0 = points[i - 1] ?? points[i]!;
        const p1 = points[i]!;
        const p2 = points[i + 1]!;
        const p3 = points[i + 2] ?? p2;
        const c1x = p1[0] + (p2[0] - p0[0]) / 6;
        const c1y = clampY(p1[1] + (p2[1] - p0[1]) / 6);
        const c2x = p2[0] - (p3[0] - p1[0]) / 6;
        const c2y = clampY(p2[1] - (p3[1] - p1[1]) / 6);
        curve += `C${fmt(c1x)},${fmt(c1y)} ${fmt(c2x)},${fmt(c2y)} ${fmt(p2[0])},${fmt(p2[1])}`;
    }
    const start = `M0,${fmt(first[1])}`;
    const end = `L${width},${fmt(last[1])}`;
    return {
        line: `${start}${curve}${end}`,
        area: `M0,${height}L0,${fmt(first[1])}${curve}${end}L${width},${height}Z`,
    };
}
