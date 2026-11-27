import { describe, expect, test } from 'bun:test';
import {
    PRESENCE_INPUT_EVENTS,
    PRESENCE_MOUSE_MOVE_MIN_PX,
    PresenceTracker,
    isPlaybackActive,
    type PresenceInputEvent,
} from '../app/utils/presenceTracker';

const SECOND = 1_000;
const MINUTE = 60 * SECOND;

// A tracker on a fake clock, with the heartbeats it sent (time, idle seconds).
function setup(timings: { heartbeatMs?: number; idleAfterMs?: number } = {}) {
    let now = 1_000_000;
    const sent: { at: number; idle: number }[] = [];
    const tracker = new PresenceTracker({
        now: () => now,
        send: (idle) => sent.push({ at: now, idle }),
        heartbeatMs: timings.heartbeatMs ?? MINUTE,
        idleAfterMs: timings.idleAfterMs ?? 5 * MINUTE,
    });
    const advance = (ms: number) => {
        now += ms;
    };
    // advances by steps of the store tick, running it
    const run = (ms: number, step = 5 * SECOND) => {
        for (let t = 0; t < ms; t += step) {
            advance(step);
            tracker.tick();
        }
    };
    return { tracker, sent, advance, run, now: () => now };
}

const key: PresenceInputEvent = { type: 'keydown', isTrusted: true };
const move = (x: number, y: number, isTrusted = true): PresenceInputEvent => ({
    type: 'mousemove',
    isTrusted,
    screenX: x,
    screenY: y,
});

describe('which events count', () => {
    test('trusted input in a visible tab', () => {
        const { tracker } = setup();
        for (const type of ['pointerdown', 'keydown', 'wheel', 'touchstart']) {
            expect(tracker.handleEvent({ type, isTrusted: true }, true)).toBe(true);
        }
        expect(PRESENCE_INPUT_EVENTS).not.toContain('scroll');
        expect(PRESENCE_INPUT_EVENTS).not.toContain('visibilitychange');
        expect(PRESENCE_INPUT_EVENTS).not.toContain('focus');
    });

    test('not in a hidden tab, not dispatched by a script', () => {
        const { tracker, sent } = setup();
        expect(tracker.handleEvent(key, false)).toBe(false);
        expect(tracker.handleEvent({ type: 'pointerdown', isTrusted: false }, true)).toBe(false);
        expect(sent).toEqual([]);
    });

    test('a mouse move counts only once the pointer really moved', () => {
        const { tracker } = setup();
        // the first position is only remembered
        expect(tracker.handleEvent(move(100, 100), true)).toBe(false);
        // fired by the browser for a layout change under a still pointer
        expect(tracker.handleEvent(move(100, 100), true)).toBe(false);
        // jitter
        expect(tracker.handleEvent(move(103, 102), true)).toBe(false);
        expect(tracker.handleEvent(move(100 + PRESENCE_MOUSE_MOVE_MIN_PX, 100), true)).toBe(true);
        // measured from the last counted position: a slow move adds up
        expect(tracker.handleEvent(move(118, 100), true)).toBe(false);
        expect(tracker.handleEvent(move(124, 100), true)).toBe(true);
        expect(tracker.handleEvent(move(300, 300, false), true)).toBe(false);
        expect(tracker.handleEvent(move(300, 300), false)).toBe(false);
        expect(tracker.handleEvent({ type: 'mousemove', isTrusted: true }, true)).toBe(false);
    });
});

describe('throttling', () => {
    test('the first input is sent at once, the next ones at the end of the period', () => {
        const { tracker, sent, advance, run } = setup();
        tracker.handleEvent(key, true);
        expect(sent).toHaveLength(1);
        expect(sent[0]!.idle).toBe(0);

        advance(10 * SECOND);
        tracker.handleEvent(key, true);
        advance(10 * SECOND);
        tracker.handleEvent(key, true);
        expect(sent).toHaveLength(1);
        expect(tracker.hasPending).toBe(true);

        // trailing heartbeat with the time since the last input
        run(40 * SECOND);
        expect(sent).toHaveLength(2);
        expect(sent[1]!.idle).toBe(40);
        expect(tracker.hasPending).toBe(false);

        // no input: nothing more
        run(10 * MINUTE);
        expect(sent).toHaveLength(2);
    });

    test('continuous input: one heartbeat per period', () => {
        const { tracker, sent, advance } = setup();
        for (let i = 0; i < 600; i++) {
            tracker.handleEvent(key, true);
            advance(SECOND);
            tracker.tick();
        }
        // 10 minutes of input, one heartbeat a minute
        expect(sent.length).toBeGreaterThanOrEqual(10);
        expect(sent.length).toBeLessThanOrEqual(11);
    });

    test('the period is at most half of idle_after', () => {
        const { tracker } = setup({ heartbeatMs: 10 * MINUTE, idleAfterMs: 2 * MINUTE });
        expect(tracker.throttleMs).toBe(MINUTE);
        tracker.configure({ heartbeatMs: 30 * SECOND, idleAfterMs: 5 * MINUTE });
        expect(tracker.throttleMs).toBe(30 * SECOND);
        tracker.configure({ heartbeatMs: 0 });
        expect(tracker.throttleMs).toBe(30 * SECOND);
    });
});

describe('coming back from away', () => {
    test('the input after a long pause is sent at once', () => {
        const { tracker, sent, run } = setup();
        tracker.handleEvent(key, true);
        run(6 * MINUTE);
        tracker.handleEvent(key, true);
        expect(sent).toHaveLength(2);
        expect(sent[1]!.idle).toBe(0);
    });

    test('also right after a sync that reported a long idle time', () => {
        const { tracker, sent, advance, run } = setup();
        tracker.handleEvent(key, true);
        run(10 * MINUTE);
        // the stream reconnected: the real idle time is reported
        tracker.sync();
        expect(sent).toHaveLength(2);
        expect(sent[1]!.idle).toBe(600);
        // the next input does not wait for the throttle period
        advance(2 * SECOND);
        tracker.handleEvent(key, true);
        expect(sent).toHaveLength(3);
        expect(sent[2]!.idle).toBe(0);
    });

    test('sync at start reports the page load as an activity', () => {
        const { tracker, sent, advance } = setup();
        advance(3 * SECOND);
        tracker.sync();
        expect(sent).toEqual([{ at: sent[0]!.at, idle: 3 }]);
    });
});

describe('playing videos', () => {
    test('a playing video counts as activity without input, its end too', () => {
        const { tracker, sent, run } = setup();
        let playing = true;
        const off = tracker.addPlayback(() => playing);
        tracker.sync();
        run(10 * MINUTE);
        // about one heartbeat a minute, each reporting a current activity
        expect(sent.length).toBeGreaterThanOrEqual(10);
        expect(sent.every((s) => s.idle <= 5)).toBe(true);

        playing = false;
        run(5 * SECOND);
        const stoppedAt = sent.length ? sent[sent.length - 1]!.at : 0;
        // the stop is reported (trailing), then nothing
        run(2 * MINUTE);
        const after = sent.filter((s) => s.at >= stoppedAt);
        expect(after.length).toBeGreaterThanOrEqual(1);
        const count = sent.length;
        run(10 * MINUTE);
        expect(sent).toHaveLength(count);
        // the last heartbeat points at the moment playback stopped
        const last = sent[sent.length - 1]!;
        expect(last.at - last.idle * SECOND).toBeGreaterThanOrEqual(stoppedAt - 5 * SECOND);
        off();
        expect(tracker.isPlaying()).toBe(false);
    });

    test('several players: one is enough; a failing probe is ignored', () => {
        const { tracker } = setup();
        tracker.addPlayback(() => {
            throw new Error('destroyed');
        });
        const offA = tracker.addPlayback(() => false);
        expect(tracker.isPlaying()).toBe(false);
        const offB = tracker.addPlayback(() => true);
        expect(tracker.isPlaying()).toBe(true);
        offB();
        offA();
        expect(tracker.isPlaying()).toBe(false);
    });

    test('isPlaybackActive: paused, ended and stalled players do not play', () => {
        const base = {
            paused: false,
            ended: false,
            lastProgressAt: 0,
            now: 0,
            stalledAfterMs: 30_000,
        };
        expect(isPlaybackActive(base)).toBe(true);
        expect(isPlaybackActive({ ...base, paused: true })).toBe(false);
        expect(isPlaybackActive({ ...base, ended: true })).toBe(false);
        expect(isPlaybackActive({ ...base, now: 29_999 })).toBe(true);
        expect(isPlaybackActive({ ...base, now: 30_000 })).toBe(false);
    });
});
