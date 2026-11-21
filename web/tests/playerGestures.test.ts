import { describe, expect, test } from 'bun:test';
import {
    DEFAULT_GESTURE_SEEK_STEP,
    GESTURE_DOUBLE_TAP_MS,
    GESTURE_SCRUB_FULL_WIDTH_SECONDS,
    GESTURE_SEEK_STREAK_MS,
    findGestureCue,
    formatGestureClock,
    formatGestureOffset,
    formatGestureSeek,
    gestureClampTime,
    gestureIsTap,
    gestureLockAxis,
    gestureScrubTime,
    gestureSeekForTap,
    gestureSide,
    gestureSwipeLevel,
    gestureTapZone,
    normalizeGestureSeekStep,
} from '../app/utils/playerGestures';

describe('normalizeGestureSeekStep', () => {
    test('keeps allowed steps', () => {
        for (const step of [5, 10, 15, 30]) expect(normalizeGestureSeekStep(step)).toBe(step);
    });
    test('falls back to the default', () => {
        for (const step of [0, 20, -5, undefined, null, '10']) {
            expect(normalizeGestureSeekStep(step)).toBe(DEFAULT_GESTURE_SEEK_STEP);
        }
    });
});

describe('gestureTapZone', () => {
    test('splits the player in thirds', () => {
        expect(gestureTapZone(0, 300)).toBe('left');
        expect(gestureTapZone(99, 300)).toBe('left');
        expect(gestureTapZone(150, 300)).toBe('center');
        expect(gestureTapZone(201, 300)).toBe('right');
        expect(gestureTapZone(300, 300)).toBe('right');
    });
    test('no width is the center', () => {
        expect(gestureTapZone(10, 0)).toBe('center');
    });
});

describe('gestureSide', () => {
    test('splits the player in halves', () => {
        expect(gestureSide(149, 300)).toBe('left');
        expect(gestureSide(150, 300)).toBe('right');
    });
});

describe('gestureLockAxis', () => {
    test('null under the threshold', () => {
        expect(gestureLockAxis(5, -6)).toBeNull();
    });
    test('dominant axis past the threshold', () => {
        expect(gestureLockAxis(-30, 8)).toBe('x');
        expect(gestureLockAxis(4, 25)).toBe('y');
        expect(gestureLockAxis(12, -12)).toBe('x');
    });
});

describe('gestureIsTap', () => {
    test('short and still', () => {
        expect(gestureIsTap(2, 3, 120)).toBe(true);
    });
    test('moved or held', () => {
        expect(gestureIsTap(15, 0, 120)).toBe(false);
        expect(gestureIsTap(0, 0, 600)).toBe(false);
    });
});

describe('gestureSeekForTap', () => {
    const step = 10;

    test('a lone tap does not seek', () => {
        expect(gestureSeekForTap('right', 1000, null, null, null, step)).toBeNull();
    });

    test('a double-tap on the right seeks forward', () => {
        expect(gestureSeekForTap('right', 1200, 1000, 'right', null, step)).toEqual({
            side: 'right',
            amount: 10,
            at: 1200,
        });
    });

    test('a double-tap on the left seeks back', () => {
        expect(gestureSeekForTap('left', 1200, 1000, 'left', null, 15)?.amount).toBe(-15);
    });

    test('taps too far apart are not a double-tap', () => {
        expect(
            gestureSeekForTap('right', 1001 + GESTURE_DOUBLE_TAP_MS, 1000, 'right', null, step),
        ).toBeNull();
    });

    test('taps on different sides are not a double-tap', () => {
        expect(gestureSeekForTap('right', 1200, 1000, 'left', null, step)).toBeNull();
    });

    test('the middle never seeks', () => {
        expect(gestureSeekForTap('center', 1200, 1000, 'center', null, step)).toBeNull();
        const streak = { side: 'right' as const, amount: 20, at: 1100 };
        expect(gestureSeekForTap('center', 1200, 1000, 'right', streak, step)).toBeNull();
    });

    test('taps during a streak accumulate', () => {
        let streak = gestureSeekForTap('right', 1200, 1000, 'right', null, step);
        streak = gestureSeekForTap('right', 1500, 1200, 'right', streak, step);
        streak = gestureSeekForTap('right', 1900, 1500, 'right', streak, step);
        expect(streak).toEqual({ side: 'right', amount: 30, at: 1900 });
    });

    test('a tap on the other side during a streak starts a new streak', () => {
        const streak = { side: 'right' as const, amount: 30, at: 1000 };
        expect(gestureSeekForTap('left', 1300, 1000, 'right', streak, step)).toEqual({
            side: 'left',
            amount: -10,
            at: 1300,
        });
    });

    test('an expired streak needs a new double-tap', () => {
        const streak = { side: 'right' as const, amount: 30, at: 1000 };
        const late = 1001 + GESTURE_SEEK_STREAK_MS;
        expect(gestureSeekForTap('right', late, 1000, 'right', streak, step)).toBeNull();
    });
});

describe('gestureClampTime', () => {
    test('clamps to the duration', () => {
        expect(gestureClampTime(-5, 100)).toBe(0);
        expect(gestureClampTime(105, 100)).toBe(100);
        expect(gestureClampTime(50, 100)).toBe(50);
    });
    test('unknown duration only clamps at zero', () => {
        expect(gestureClampTime(500, 0)).toBe(500);
        expect(gestureClampTime(-1, NaN)).toBe(0);
    });
});

describe('gestureScrubTime', () => {
    test('a full-width swipe covers the scrub span', () => {
        expect(gestureScrubTime(100, 400, 400, 3600)).toBe(100 + GESTURE_SCRUB_FULL_WIDTH_SECONDS);
        expect(gestureScrubTime(100, -200, 400, 3600)).toBe(
            100 - GESTURE_SCRUB_FULL_WIDTH_SECONDS / 2,
        );
    });
    test('short videos scale to their duration', () => {
        expect(gestureScrubTime(0, 200, 400, 30)).toBe(15);
    });
    test('stays within the video', () => {
        expect(gestureScrubTime(10, -400, 400, 3600)).toBe(0);
        expect(gestureScrubTime(3590, 400, 400, 3600)).toBe(3600);
    });
    test('no width or duration does not move', () => {
        expect(gestureScrubTime(42, 100, 0, 3600)).toBe(42);
        expect(gestureScrubTime(42, 100, 400, 0)).toBe(42);
    });
});

describe('gestureSwipeLevel', () => {
    test('swiping up raises the level', () => {
        expect(gestureSwipeLevel(0.5, -100, 400, 0, 1)).toBeCloseTo(0.75);
    });
    test('swiping down lowers the level', () => {
        expect(gestureSwipeLevel(0.5, 200, 400, 0, 1)).toBeCloseTo(0);
    });
    test('clamps to the range', () => {
        expect(gestureSwipeLevel(0.9, -400, 400, 0, 1)).toBe(1);
        expect(gestureSwipeLevel(1, 800, 400, 0.2, 1.5)).toBe(0.2);
    });
    test('uses the whole range over the height', () => {
        expect(gestureSwipeLevel(0.2, -400, 400, 0.2, 1.5)).toBeCloseTo(1.5);
    });
});

describe('formatting', () => {
    test('seek amounts', () => {
        expect(formatGestureSeek(30)).toBe('+30s');
        expect(formatGestureSeek(-10)).toBe('-10s');
        expect(formatGestureSeek(0)).toBe('+0s');
    });
    test('clock', () => {
        expect(formatGestureClock(0)).toBe('0:00');
        expect(formatGestureClock(65.7)).toBe('1:05');
        expect(formatGestureClock(3723)).toBe('1:02:03');
        expect(formatGestureClock(-3)).toBe('0:00');
    });
    test('offsets', () => {
        expect(formatGestureOffset(65)).toBe('+1:05');
        expect(formatGestureOffset(-12)).toBe('-0:12');
    });
});

describe('findGestureCue', () => {
    const cues = [
        { start: 0, end: 10, url: 'a' },
        { start: 10, end: 20, url: 'b' },
    ];
    test('finds the cue covering the time', () => {
        expect(findGestureCue(cues, 0)?.url).toBe('a');
        expect(findGestureCue(cues, 10)?.url).toBe('b');
    });
    test('null outside the cues', () => {
        expect(findGestureCue(cues, 20)).toBeNull();
        expect(findGestureCue([], 5)).toBeNull();
    });
});
