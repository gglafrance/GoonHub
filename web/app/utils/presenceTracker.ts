// Activity tracker of a tab for the presence (docs/PRESENCE.md, "Online and away"). Pure (clock
// and sender injected): the presence store wires it to the DOM. Tests in
// web/tests/presenceTracker.test.ts.

/**
 * Input events that count as activity. A `scroll` event is not one of them: programmatic
 * scrolls (scroll restore, infinite lists) fire it too; a scroll by the user comes with a
 * wheel, a key, a touch or a pointer down (scrollbar).
 */
export const PRESENCE_INPUT_EVENTS = [
    'pointerdown',
    'keydown',
    'wheel',
    'touchstart',
    'mousemove',
] as const;

/**
 * A mouse move counts once the pointer moved this far (screen pixels) from where it was last
 * counted: the browser fires mousemove without any move when the page changes under a still
 * pointer (layout, scroll, live updates), and a jittery mouse moves by a pixel or two.
 */
export const PRESENCE_MOUSE_MOVE_MIN_PX = 12;

/** How often the store runs `tick` (playing videos, trailing heartbeats). */
export const PRESENCE_TICK_MS = 5_000;

/** The subset of a DOM input event the tracker reads. */
export interface PresenceInputEvent {
    type: string;
    isTrusted: boolean;
    screenX?: number;
    screenY?: number;
}

export interface PresenceTrackerOptions {
    now: () => number;
    /** Sends a heartbeat: the last input (or playback) was `idleSeconds` ago. */
    send: (idleSeconds: number) => void;
    heartbeatMs?: number;
    idleAfterMs?: number;
}

/**
 * Whether a player counts as playing: not paused, not ended, and its position moved less than
 * `stalledAfterMs` ago (a buffering player is not paused but does not play).
 */
export function isPlaybackActive(state: {
    paused: boolean;
    ended: boolean;
    lastProgressAt: number;
    now: number;
    stalledAfterMs: number;
}): boolean {
    if (state.paused || state.ended) return false;
    return state.now - state.lastProgressAt < state.stalledAfterMs;
}

/**
 * Tracks the real activity of a tab (input, playing videos) and reports it with heartbeats:
 * - an input sends a heartbeat at once when none was sent for the throttle period, else one is
 *   sent at the end of it (trailing, with the time since the last input), so the server always
 *   knows the real last input within one period;
 * - when this tab may be shown away (its last reported activity is about `idleAfterMs` old), the
 *   next input is sent at once, without waiting for the throttle;
 * - a playing video (any registered probe) counts as activity at every tick, and the moment
 *   playback stops too (away counts from then);
 * - `sync` reports the real idle time at once (start, stream reconnected).
 */
export class PresenceTracker {
    private readonly now: () => number;
    private readonly sendFn: (idleSeconds: number) => void;
    private heartbeatMs: number;
    private idleAfterMs: number;
    private lastActivityAt: number;
    private lastSentAt = Number.NEGATIVE_INFINITY;
    private reportedActivityAt = Number.NEGATIVE_INFINITY;
    private pending = false;
    private anchor: { x: number; y: number } | null = null;
    private readonly probes = new Set<() => boolean>();
    private wasPlaying = false;

    constructor(options: PresenceTrackerOptions) {
        this.now = options.now;
        this.sendFn = options.send;
        this.heartbeatMs = options.heartbeatMs ?? 60_000;
        this.idleAfterMs = options.idleAfterMs ?? 300_000;
        // opening the page is an action of the user
        this.lastActivityAt = this.now();
    }

    configure(timings: { heartbeatMs?: number; idleAfterMs?: number }) {
        if (timings.heartbeatMs && timings.heartbeatMs > 0) this.heartbeatMs = timings.heartbeatMs;
        if (timings.idleAfterMs && timings.idleAfterMs > 0) this.idleAfterMs = timings.idleAfterMs;
    }

    /** At most one heartbeat per period (at most half of idle_after, at least 1 s). */
    get throttleMs(): number {
        return Math.max(1_000, Math.min(this.heartbeatMs, this.idleAfterMs / 2));
    }

    /** Whether a heartbeat waits for the end of the throttle period. */
    get hasPending(): boolean {
        return this.pending;
    }

    /**
     * An input event of the window. Counts only when trusted (not dispatched by a script), in a
     * visible tab, and for mouse moves only once the pointer really moved. Returns whether it
     * counted.
     */
    handleEvent(event: PresenceInputEvent, visible: boolean): boolean {
        if (!event.isTrusted || !visible) return false;
        if (event.type === 'mousemove') {
            if (event.screenX === undefined || event.screenY === undefined) return false;
            const at = { x: event.screenX, y: event.screenY };
            if (!this.anchor) {
                this.anchor = at;
                return false;
            }
            const moved = Math.hypot(at.x - this.anchor.x, at.y - this.anchor.y);
            if (moved < PRESENCE_MOUSE_MOVE_MIN_PX) return false;
            this.anchor = at;
        }
        this.activity();
        return true;
    }

    /** Registers a player: the probe tells whether it plays. Returns the unregister function. */
    addPlayback(probe: () => boolean): () => void {
        this.probes.add(probe);
        return () => {
            this.probes.delete(probe);
        };
    }

    /** Whether a registered player plays. */
    isPlaying(): boolean {
        for (const probe of this.probes) {
            try {
                if (probe()) return true;
            } catch {
                // a player being destroyed
            }
        }
        return false;
    }

    /** Periodic work: playing videos count as activity, trailing heartbeats are sent. */
    tick() {
        const playing = this.isPlaying();
        if (playing || this.wasPlaying) this.activity();
        this.wasPlaying = playing;
        if (this.pending && this.now() - this.lastSentAt >= this.throttleMs) this.send();
    }

    /** Reports the real idle time at once (page start, live stream reconnected). */
    sync() {
        if (this.isPlaying()) this.lastActivityAt = this.now();
        this.send();
    }

    /** Something the user did (or a playing video) now. */
    activity() {
        const now = this.now();
        this.lastActivityAt = now;
        const throttled = now - this.lastSentAt < this.throttleMs;
        // the server may show this tab's user away (or will before a trailing heartbeat)
        const mayBeAway = now - this.reportedActivityAt >= this.idleAfterMs - this.throttleMs;
        if (!throttled || mayBeAway) this.send();
        else this.pending = true;
    }

    private send() {
        const now = this.now();
        const idleSeconds = Math.max(0, Math.round((now - this.lastActivityAt) / 100) / 10);
        this.lastSentAt = now;
        this.reportedActivityAt = this.lastActivityAt;
        this.pending = false;
        this.sendFn(idleSeconds);
    }
}
