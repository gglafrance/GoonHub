import { describe, expect, test } from 'bun:test';
import {
    SCRUBBER_FOLLOW_LEAD,
    SCRUBBER_FOLLOW_RESUME_MS,
    SCRUBBER_MIN_VELOCITY,
    adjacentCueIndex,
    clampScroll,
    decayVelocity,
    findCueIndex,
    followScrollTarget,
    formatScrubberTime,
    layoutChips,
    markerChipWidth,
    maxScroll,
    offsetToTime,
    pageScrollTarget,
    releaseVelocity,
    scrubberTileWidth,
    shouldResumeFollow,
    smoothScrollStep,
    tileIndexAt,
    timeToOffset,
    visibleTileRange,
    wheelScrollDelta,
} from '../app/utils/scrubber';

// 10 second cues: [0,10), [10,20), ...
const cues = (n: number, step = 10) =>
    Array.from({ length: n }, (_, i) => ({ start: i * step, end: (i + 1) * step }));

describe('scrubberTileWidth', () => {
    test('follows the sprite aspect ratio', () => {
        expect(scrubberTileWidth(160, 90, 64)).toBe(114);
        expect(scrubberTileWidth(90, 160, 64)).toBe(36);
    });
    test('falls back to 16:9 without a size', () => {
        expect(scrubberTileWidth(0, 0, 90)).toBe(160);
    });
});

describe('findCueIndex', () => {
    const list = cues(5);
    test('finds the cue of a time', () => {
        expect(findCueIndex(list, 0)).toBe(0);
        expect(findCueIndex(list, 9.99)).toBe(0);
        expect(findCueIndex(list, 10)).toBe(1);
        expect(findCueIndex(list, 47)).toBe(4);
    });
    test('clamps outside the cues', () => {
        expect(findCueIndex(list, -5)).toBe(0);
        expect(findCueIndex(list, 500)).toBe(4);
        expect(findCueIndex(list, Number.NaN)).toBe(0);
    });
    test('no cues', () => {
        expect(findCueIndex([], 10)).toBe(-1);
    });
    test('handles thousands of cues', () => {
        const long = cues(5000, 2);
        expect(findCueIndex(long, 7777)).toBe(3888);
    });
});

describe('time <-> offset', () => {
    const list = cues(4);
    test('time to offset is linear inside a thumbnail', () => {
        expect(timeToOffset(list, 0, 100)).toBe(0);
        expect(timeToOffset(list, 5, 100)).toBe(50);
        expect(timeToOffset(list, 25, 100)).toBe(250);
    });
    test('time past the last cue stops at the end', () => {
        expect(timeToOffset(list, 1000, 100)).toBe(400);
    });
    test('offset to time is the inverse', () => {
        for (const t of [0, 3, 10, 17.5, 39]) {
            expect(offsetToTime(list, timeToOffset(list, t, 100), 100)).toBeCloseTo(t, 6);
        }
    });
    test('offset to time clamps', () => {
        expect(offsetToTime(list, -50, 100)).toBe(0);
        expect(offsetToTime(list, 9999, 100)).toBe(40);
        expect(offsetToTime([], 50, 100)).toBe(0);
    });
    test('uneven cues', () => {
        const uneven = [
            { start: 0, end: 4 },
            { start: 4, end: 20 },
        ];
        expect(timeToOffset(uneven, 12, 100)).toBe(150);
        expect(offsetToTime(uneven, 150, 100)).toBe(12);
    });
});

describe('tileIndexAt', () => {
    test('maps a position to a thumbnail', () => {
        expect(tileIndexAt(0, 100, 3)).toBe(0);
        expect(tileIndexAt(199, 100, 3)).toBe(1);
        expect(tileIndexAt(300, 100, 3)).toBe(-1);
        expect(tileIndexAt(-1, 100, 3)).toBe(-1);
    });
});

describe('visibleTileRange', () => {
    test('renders the visible thumbnails and the overscan', () => {
        expect(visibleTileRange(0, 500, 100, 1000, 2)).toEqual({ first: 0, last: 6 });
        expect(visibleTileRange(1050, 500, 100, 1000, 2)).toEqual({ first: 8, last: 17 });
    });
    test('stays inside the cues', () => {
        expect(visibleTileRange(99_000, 1000, 100, 1000, 4)).toEqual({ first: 986, last: 999 });
        expect(visibleTileRange(0, 5000, 100, 3, 4)).toEqual({ first: 0, last: 2 });
    });
    test('renders only a few nodes for thousands of cues', () => {
        const { first, last } = visibleTileRange(250_000, 1200, 114, 6000);
        expect(last - first + 1).toBeLessThan(25);
    });
    test('nothing to render', () => {
        const empty = visibleTileRange(0, 500, 100, 0);
        expect(empty.last).toBeLessThan(empty.first);
    });
});

describe('scroll bounds', () => {
    test('clamps to the content', () => {
        expect(maxScroll(500, 2000)).toBe(1500);
        expect(maxScroll(500, 300)).toBe(0);
        expect(clampScroll(-10, 500, 2000)).toBe(0);
        expect(clampScroll(1800, 500, 2000)).toBe(1500);
    });
    test('page buttons', () => {
        expect(pageScrollTarget(0, 1000, 10_000, 1)).toBe(800);
        expect(pageScrollTarget(500, 1000, 10_000, -1)).toBe(0);
        expect(pageScrollTarget(8800, 1000, 10_000, 1)).toBe(9000);
    });
});

describe('followScrollTarget', () => {
    test('no scroll while the position is in the comfortable zone', () => {
        expect(followScrollTarget(500, 0, 1000, 10_000)).toBeNull();
    });
    test('brings the position back to the lead when it leaves the zone', () => {
        expect(followScrollTarget(900, 0, 1000, 10_000)).toBe(900 - 1000 * SCRUBBER_FOLLOW_LEAD);
        expect(followScrollTarget(3000, 0, 1000, 10_000)).toBe(2750);
        // seek backwards
        expect(followScrollTarget(100, 5000, 1000, 10_000)).toBe(0);
    });
    test('stays inside the content', () => {
        expect(followScrollTarget(9990, 0, 1000, 10_000)).toBe(9000);
        // short content: nothing to scroll
        expect(followScrollTarget(900, 0, 1000, 800)).toBeNull();
    });
});

describe('shouldResumeFollow', () => {
    test('resumes after the idle delay while playing', () => {
        expect(shouldResumeFollow(SCRUBBER_FOLLOW_RESUME_MS, true, false)).toBe(true);
        expect(shouldResumeFollow(SCRUBBER_FOLLOW_RESUME_MS - 1, true, false)).toBe(false);
    });
    test('not while paused or while the user still moves the strip', () => {
        expect(shouldResumeFollow(60_000, false, false)).toBe(false);
        expect(shouldResumeFollow(60_000, true, true)).toBe(false);
    });
});

describe('wheelScrollDelta', () => {
    test('vertical wheel scrolls sideways', () => {
        expect(wheelScrollDelta(0, 100, 0, 800)).toBe(100);
        expect(wheelScrollDelta(0, -3, 1, 800)).toBe(-120);
        expect(wheelScrollDelta(0, 1, 2, 800)).toBe(800);
    });
    test('horizontal trackpad swipe wins when dominant', () => {
        expect(wheelScrollDelta(-30, 4, 0, 800)).toBe(-30);
    });
});

describe('drag inertia', () => {
    test('release velocity is opposite to the pointer movement', () => {
        const samples = [
            { x: 500, t: 0 },
            { x: 450, t: 50 },
            { x: 400, t: 100 },
        ];
        expect(releaseVelocity(samples)).toBe(1);
    });
    test('only recent samples count', () => {
        const samples = [
            { x: 0, t: 0 },
            { x: 0, t: 1000 },
            { x: 20, t: 1020 },
        ];
        expect(releaseVelocity(samples)).toBe(-1);
        expect(releaseVelocity([{ x: 0, t: 0 }])).toBe(0);
    });
    test('velocity decays then stops', () => {
        let v = 2;
        let frames = 0;
        while (v !== 0 && frames < 1000) {
            v = decayVelocity(v, 16);
            frames++;
        }
        expect(v).toBe(0);
        expect(frames).toBeGreaterThan(10);
        expect(frames).toBeLessThan(200);
        expect(decayVelocity(SCRUBBER_MIN_VELOCITY / 2, 16)).toBe(0);
    });
    test('smooth scroll reaches its target', () => {
        let x = 0;
        for (let i = 0; i < 100 && x !== 1000; i++) x = smoothScrollStep(x, 1000);
        expect(x).toBe(1000);
    });
});

describe('adjacentCueIndex', () => {
    const list = cues(5);
    test('previous and next thumbnail', () => {
        expect(adjacentCueIndex(list, 15, 1)).toBe(2);
        expect(adjacentCueIndex(list, 15, -1)).toBe(0);
    });
    test('a time a hair before a cue start counts as that cue', () => {
        expect(adjacentCueIndex(list, 19.99, 1)).toBe(3);
    });
    test('stays inside the cues', () => {
        expect(adjacentCueIndex(list, 0, -1)).toBe(0);
        expect(adjacentCueIndex(list, 45, 1)).toBe(4);
        expect(adjacentCueIndex([], 0, 1)).toBe(-1);
    });
});

describe('formatScrubberTime', () => {
    test('formats clocks', () => {
        expect(formatScrubberTime(0)).toBe('0:00');
        expect(formatScrubberTime(65.9)).toBe('1:05');
        expect(formatScrubberTime(3600 + 62)).toBe('1:01:02');
        expect(formatScrubberTime(Number.NaN)).toBe('0:00');
    });
});

describe('marker chips', () => {
    test('overlapping chips are shifted right', () => {
        const items = [
            { x: 100, width: 50 },
            { x: 0, width: 50 },
            { x: 120, width: 50 },
        ];
        expect(layoutChips(items, Infinity, 2)).toEqual([100, 0, 152]);
    });
    test('chips stay inside the content', () => {
        expect(layoutChips([{ x: 990, width: 50 }], 1000)).toEqual([950]);
    });
    test('width grows with the label up to a maximum', () => {
        expect(markerChipWidth('ab')).toBeLessThan(markerChipWidth('abcdef'));
        expect(markerChipWidth('x'.repeat(200))).toBe(140);
    });
});
