/** Most updates a second when the video frames cannot be followed (no requestVideoFrameCallback). */
export const PLAYBACK_LOOP_MAX_FPS = 30;

// Missing in some browsers (Firefox before 132): the type says it is always there
function hasFrameCallback(video: HTMLVideoElement | null): video is HTMLVideoElement {
    return !!video && typeof video.requestVideoFrameCallback === 'function';
}

/**
 * Runs `tick` while a video plays, once per video frame shown (requestVideoFrameCallback), or
 * at most PLAYBACK_LOOP_MAX_FPS times a second with requestAnimationFrame where that is missing.
 * A requestAnimationFrame loop runs at the display rate: on a 360 Hz screen the scrubber strips
 * were updated 360 times a second for a 30 fps video, which took a whole CPU core per watch tab.
 * start() and stop() can be called repeatedly; stop() is also called on unmount.
 */
export function usePlaybackFrameLoop(getVideo: () => HTMLVideoElement | null, tick: () => void) {
    let running = false;
    let video: HTMLVideoElement | null = null;
    let frameHandle = 0;
    let rafId = 0;
    let last = 0;
    const minInterval = 1000 / PLAYBACK_LOOP_MAX_FPS;

    const onVideoFrame = () => {
        if (!running || !video) return;
        tick();
        frameHandle = video.requestVideoFrameCallback(onVideoFrame);
    };

    const onAnimationFrame = (now: number) => {
        if (!running) return;
        if (now - last >= minInterval) {
            last = now;
            tick();
        }
        rafId = requestAnimationFrame(onAnimationFrame);
    };

    function start() {
        if (running) return;
        running = true;
        const el = getVideo();
        if (hasFrameCallback(el)) {
            video = el;
            frameHandle = el.requestVideoFrameCallback(onVideoFrame);
        } else {
            last = 0;
            rafId = requestAnimationFrame(onAnimationFrame);
        }
    }

    function stop() {
        running = false;
        if (video && frameHandle) video.cancelVideoFrameCallback(frameHandle);
        if (rafId) cancelAnimationFrame(rafId);
        video = null;
        frameHandle = 0;
        rafId = 0;
    }

    onBeforeUnmount(stop);

    return { start, stop };
}
