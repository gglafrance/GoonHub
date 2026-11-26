import { describe, expect, test } from 'bun:test';
import { scrubberWheelDelta } from '../app/utils/scrubber';

const wheel = (deltaX: number, deltaY: number, deltaMode = 0, ctrlKey = false) => ({
    deltaX,
    deltaY,
    deltaMode,
    ctrlKey,
});

describe('scrubberWheelDelta', () => {
    test('off (the default): the wheel is left to the page', () => {
        expect(scrubberWheelDelta(wheel(0, 120), false, 800)).toBe(0);
        expect(scrubberWheelDelta(wheel(-60, 0), false, 800)).toBe(0);
        expect(scrubberWheelDelta(wheel(0, 3, 1), false, 800)).toBe(0);
    });
    test('on: the dominant axis scrolls the strip, lines and pages in pixels', () => {
        expect(scrubberWheelDelta(wheel(0, 120), true, 800)).toBe(120);
        expect(scrubberWheelDelta(wheel(-60, 10), true, 800)).toBe(-60);
        expect(scrubberWheelDelta(wheel(0, 3, 1), true, 800)).toBe(120);
        expect(scrubberWheelDelta(wheel(0, 1, 2), true, 800)).toBe(800);
    });
    test('ctrl (pinch zoom) is never handled', () => {
        expect(scrubberWheelDelta(wheel(0, 120, 0, true), true, 800)).toBe(0);
    });
});
