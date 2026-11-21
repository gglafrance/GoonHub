export interface ScrollTarget {
    left: number;
    top: number;
}

export interface ScrollRestoreOptions {
    /** Give up waiting for the content after this delay (ms) and scroll as far as possible. */
    timeout?: number;
    /** Keep the position for this long (ms) after the restore, while late content renders. */
    settle?: number;
    /** Abort when this returns false (e.g. another navigation started). */
    isCurrent?: () => boolean;
}

/** Max vertical scroll of the document. */
function maxScrollTop(): number {
    const el = document.scrollingElement ?? document.documentElement;
    return Math.max(0, el.scrollHeight - window.innerHeight);
}

/** Whether a saved position can be reached with the current document height. */
export function isScrollReachable(targetTop: number, maxTop: number): boolean {
    return targetTop <= maxTop + 1;
}

/**
 * Next frame, or a short timer when frames do not come (hidden or off-screen tab, where
 * animation frames are paused): the callback runs once, whichever comes first.
 */
export function nextFrame(fn: () => void) {
    let done = false;
    const run = () => {
        if (done) return;
        done = true;
        fn();
    };
    requestAnimationFrame(run);
    setTimeout(run, 50);
}

const USER_SCROLL_EVENTS = ['wheel', 'touchstart', 'keydown', 'mousedown'] as const;

/**
 * Waits until the page is tall enough to reach a saved scroll position (the list data loads
 * after the page is mounted), then resolves with it. While the content keeps rendering (late
 * sections, images without a fixed size), the position is held for a short settle time.
 * Resolves with false when the user scrolls by themselves, or when the navigation is outdated.
 */
export function waitForScrollTarget(
    target: ScrollTarget,
    { timeout = 5000, settle = 1500, isCurrent = () => true }: ScrollRestoreOptions = {},
): Promise<ScrollTarget | false> {
    return new Promise((resolve) => {
        let userScrolled = false;
        const onUser = () => {
            userScrolled = true;
        };
        for (const ev of USER_SCROLL_EVENTS) {
            window.addEventListener(ev, onUser, { passive: true, capture: true });
        }
        const cleanup = () => {
            for (const ev of USER_SCROLL_EVENTS) {
                window.removeEventListener(ev, onUser, { capture: true });
            }
        };

        const start = performance.now();
        let restoredAt = 0;

        const tick = () => {
            if (userScrolled || !isCurrent()) {
                cleanup();
                if (!restoredAt) resolve(false);
                return;
            }
            const now = performance.now();
            if (!restoredAt) {
                if (isScrollReachable(target.top, maxScrollTop()) || now - start > timeout) {
                    restoredAt = now;
                    // The router scrolls to the resolved position
                    resolve(target);
                }
            } else {
                // Content rendered late can shift the page: hold the position until it settles
                if (now - restoredAt > settle) {
                    cleanup();
                    return;
                }
                if (
                    Math.abs(window.scrollY - target.top) > 2 &&
                    isScrollReachable(target.top, maxScrollTop())
                ) {
                    window.scrollTo(target.left, target.top);
                }
            }
            nextFrame(tick);
        };
        nextFrame(tick);
    });
}

// Position saved in the history entry the app was (re)loaded on. Nuxt hands it to the first
// navigation after the load, where it does not belong: see plugins/scroll-restore.client.ts.
let loadPosition: ScrollTarget | null = null;

export function setLoadScrollPosition(position: ScrollTarget | null) {
    loadPosition = position;
}

/** Whether a saved position given to the router is the one of the page load (consumed once). */
export function isLoadScrollPosition(position: { left?: number; top?: number }): boolean {
    const match =
        !!loadPosition &&
        (position.left ?? 0) === loadPosition.left &&
        (position.top ?? 0) === loadPosition.top;
    loadPosition = null;
    return match;
}
