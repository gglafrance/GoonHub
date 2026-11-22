import type videojs from 'video.js';

type Player = ReturnType<typeof videojs>;

interface Options {
    player: () => Player | null;
    // the player plays the file itself (no conversion on the fly): the video element can be
    // seeked to a keyframe directly (fastSeek), the conversion middleware is not in the way
    directStream: () => boolean;
}

// Continuous seeking of the centered scrubber while the strip is dragged or wheeled: one seek at
// a time (the next target waits for the previous seek to finish, checked every frame), fast
// keyframe seeks where the browser has them, and a precise seek at the end. Seeks go through the
// player API like the progress bar, so a converted stream handles its start offset (and debounces
// the reload of the stream). The decisions live in utils/scrubber.ts.
export function useScrubberSeek(options: Options) {
    let target = Number.NaN;
    let lastIssued = Number.NaN;
    let lastWasFast = false;
    let lastIssueAt = 0;
    let rafId = 0;
    let listening: Player | null = null;

    const videoElement = (p: Player): HTMLVideoElement | null => {
        const el = p.tech(true)?.el();
        return el instanceof HTMLVideoElement ? el : null;
    };

    const issue = (p: Player, time: number, fast: boolean) => {
        const el = fast && options.directStream() ? videoElement(p) : null;
        if (el && typeof el.fastSeek === 'function') {
            el.fastSeek(time);
            lastWasFast = true;
        } else {
            p.currentTime(time);
            lastWasFast = false;
        }
        lastIssued = time;
        lastIssueAt = performance.now();
    };

    const cancelFrame = () => {
        if (rafId) cancelAnimationFrame(rafId);
        rafId = 0;
    };

    function pump() {
        rafId = 0;
        const p = options.player();
        if (!p || !Number.isFinite(target)) return;
        const since = performance.now() - lastIssueAt;
        if (shouldIssueSeek(target, lastIssued, p.seeking(), since)) issue(p, target, true);
        // Not there yet (a seek still running): look again next frame
        const behind = !(Math.abs(target - lastIssued) < SCRUBBER_SEEK_MIN_DELTA);
        if (behind) schedule();
    }

    const schedule = () => {
        if (!rafId) rafId = requestAnimationFrame(pump);
    };

    // A seek finished: the next target can go at once
    const onSeeked = () => {
        if (Number.isFinite(target)) {
            cancelFrame();
            pump();
        }
    };

    const listen = (p: Player | null) => {
        if (listening === p) return;
        listening?.off('seeked', onSeeked);
        listening = p;
        p?.on('seeked', onSeeked);
    };

    // The strip moved: seek towards `time` (throttled)
    const request = (time: number) => {
        listen(options.player());
        target = time;
        schedule();
    };

    // The drag or wheel ended on `time`: a precise seek when the last one was not exactly there
    const finish = (time: number) => {
        cancelFrame();
        const p = options.player();
        if (p && needsFinalSeek(time, lastIssued, lastWasFast)) issue(p, time, false);
        target = Number.NaN;
        lastIssued = Number.NaN;
        lastWasFast = false;
    };

    // Whether a drag or wheel seek is under way
    const active = () => Number.isFinite(target);

    onBeforeUnmount(() => {
        cancelFrame();
        listen(null);
    });

    return { request, finish, active };
}
