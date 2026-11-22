import { describe, expect, test } from 'bun:test';
import {
    MARKER_TIME_FORMAT_HINT,
    formatMarkerTime,
    nudgeMarkerTime,
    parseMarkerTime,
    validateMarkerEnd,
    validateMarkerStart,
    validateMarkerTimeEdit,
} from '../app/utils/markerTime';

describe('parseMarkerTime', () => {
    test('seconds alone', () => {
        expect(parseMarkerTime('0')).toBe(0);
        expect(parseMarkerTime('42')).toBe(42);
        // Past a minute is fine with seconds alone
        expect(parseMarkerTime('90')).toBe(90);
    });
    test('mm:ss', () => {
        expect(parseMarkerTime('1:15')).toBe(75);
        expect(parseMarkerTime('12:30')).toBe(750);
        expect(parseMarkerTime('00:05')).toBe(5);
        // The first part may pass 59
        expect(parseMarkerTime('75:00')).toBe(4500);
    });
    test('h:mm:ss', () => {
        expect(parseMarkerTime('1:02:03')).toBe(3723);
        expect(parseMarkerTime('0:00:00')).toBe(0);
    });
    test('spaces around the text and the parts are ignored', () => {
        expect(parseMarkerTime('  12:30 ')).toBe(750);
        expect(parseMarkerTime('12 : 30')).toBe(750);
    });
    test('anything else is not a time', () => {
        for (const text of [
            '',
            '   ',
            'abc',
            '12a',
            '-5',
            '1.5',
            '1:60',
            '1:75:00',
            '1:2:3:4',
            ':30',
            '12:',
            '1::2',
            '1:005',
        ]) {
            expect(parseMarkerTime(text)).toBeNull();
        }
    });
});

describe('formatMarkerTime', () => {
    test('mm:ss, h:mm:ss past an hour', () => {
        expect(formatMarkerTime(0)).toBe('00:00');
        expect(formatMarkerTime(75)).toBe('01:15');
        expect(formatMarkerTime(3723)).toBe('1:02:03');
    });
    test('fractions are dropped, negatives and NaN are 0', () => {
        expect(formatMarkerTime(75.9)).toBe('01:15');
        expect(formatMarkerTime(-3)).toBe('00:00');
        expect(formatMarkerTime(Number.NaN)).toBe('00:00');
    });
    test('round trips with parseMarkerTime', () => {
        for (const s of [0, 5, 59, 60, 3599, 3600, 7322]) {
            expect(parseMarkerTime(formatMarkerTime(s))).toBe(s);
        }
    });
});

describe('validateMarkerStart', () => {
    test('a start within the scene and before the end is fine', () => {
        expect(validateMarkerStart(0, null, 600)).toBeNull();
        expect(validateMarkerStart(60, 90, 600)).toBeNull();
        expect(validateMarkerStart(89, 90, 600)).toBeNull();
    });
    test('like the API: not negative', () => {
        expect(validateMarkerStart(-1, null, 600)).toBe('The start cannot be negative');
    });
    test('like the API: before the end', () => {
        expect(validateMarkerStart(90, 90, 600)).toBe('The start must be before the end (01:30)');
        expect(validateMarkerStart(120, 90, 600)).toBe('The start must be before the end (01:30)');
    });
    test('like the API: within the scene (the duration itself is accepted)', () => {
        expect(validateMarkerStart(600, null, 600)).toBeNull();
        expect(validateMarkerStart(601, null, 600)).toBe(
            'The start is past the end of the scene (10:00)',
        );
    });
    test('an unknown duration is not checked', () => {
        expect(validateMarkerStart(99999, null, 0)).toBeNull();
        expect(validateMarkerStart(99999, null, null)).toBeNull();
    });
});

describe('validateMarkerEnd', () => {
    test('an end after the start and within the scene is fine', () => {
        expect(validateMarkerEnd(60, 61, 600)).toBeNull();
        expect(validateMarkerEnd(60, 600, 600)).toBeNull();
    });
    test('like the API: after the start', () => {
        expect(validateMarkerEnd(60, 60, 600)).toBe('The end must be after the start (01:00)');
        expect(validateMarkerEnd(60, 30, 600)).toBe('The end must be after the start (01:00)');
    });
    test('like the API: within the scene', () => {
        expect(validateMarkerEnd(60, 601, 600)).toBe(
            'The end is past the end of the scene (10:00)',
        );
        expect(validateMarkerEnd(60, 601, null)).toBeNull();
    });
});

describe('validateMarkerTimeEdit', () => {
    const point = { timestamp: 60, end_timestamp: null };
    const range = { timestamp: 60, end_timestamp: 90 };
    test('an unreadable text asks for the format', () => {
        expect(validateMarkerTimeEdit('start', null, point, 600)).toBe(MARKER_TIME_FORMAT_HINT);
        expect(validateMarkerTimeEdit('end', null, range, 600)).toBe(MARKER_TIME_FORMAT_HINT);
    });
    test('the start is checked against the end of the marker', () => {
        expect(validateMarkerTimeEdit('start', 100, point, 600)).toBeNull();
        expect(validateMarkerTimeEdit('start', 100, range, 600)).toBe(
            'The start must be before the end (01:30)',
        );
    });
    test('the end is checked against the start of the marker', () => {
        expect(validateMarkerTimeEdit('end', 75, range, 600)).toBeNull();
        expect(validateMarkerTimeEdit('end', 50, range, 600)).toBe(
            'The end must be after the start (01:00)',
        );
        expect(validateMarkerTimeEdit('end', 700, range, 600)).toBe(
            'The end is past the end of the scene (10:00)',
        );
    });
});

describe('nudgeMarkerTime', () => {
    test('moves the typed time by a second', () => {
        expect(nudgeMarkerTime('12:30', 1, 0, 3600)).toBe('12:31');
        expect(nudgeMarkerTime('12:30', -1, 0, 3600)).toBe('12:29');
        expect(nudgeMarkerTime('59:59', 1, 0, 7200)).toBe('1:00:00');
        // Any typed format comes back as mm:ss
        expect(nudgeMarkerTime('90', 1, 0, 3600)).toBe('01:31');
    });
    test('stays between 0 and the scene duration', () => {
        expect(nudgeMarkerTime('00:00', -1, 0, 600)).toBe('00:00');
        expect(nudgeMarkerTime('10:00', 1, 0, 600)).toBe('10:00');
        expect(nudgeMarkerTime('10:00', 1, 0, null)).toBe('10:01');
    });
    test('an unreadable text starts from the fallback', () => {
        expect(nudgeMarkerTime('oops', 1, 75, 600)).toBe('01:16');
    });
});
