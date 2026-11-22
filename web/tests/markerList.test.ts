import { describe, expect, test } from 'bun:test';
import {
    groupMarkersByLabel,
    markerListGroups,
    markerTabLabel,
    parseMarkerListSort,
    sortMarkersByTime,
} from '../app/utils/markerList';

const m = (id: number, timestamp: number, label = '') => ({ id, timestamp, label });

const ids = (list: { id: number }[]) => list.map((x) => x.id);

describe('markerTabLabel', () => {
    test('count suffix when the scene has markers', () => {
        expect(markerTabLabel(1)).toBe('Markers (1)');
        expect(markerTabLabel(12)).toBe('Markers (12)');
    });
    test('no suffix without markers', () => {
        expect(markerTabLabel(0)).toBe('Markers');
    });
});

describe('parseMarkerListSort', () => {
    test('known values', () => {
        expect(parseMarkerListSort('label')).toBe('label');
        expect(parseMarkerListSort('time')).toBe('time');
    });
    test('missing or unknown values fall back to time', () => {
        expect(parseMarkerListSort(null)).toBe('time');
        expect(parseMarkerListSort(undefined)).toBe('time');
        expect(parseMarkerListSort('bogus')).toBe('time');
    });
});

describe('sortMarkersByTime', () => {
    test('by start, ties by id, without changing the input', () => {
        const input = [m(3, 50), m(2, 10), m(1, 50), m(4, 0)];
        expect(ids(sortMarkersByTime(input))).toEqual([4, 2, 1, 3]);
        expect(ids(input)).toEqual([3, 2, 1, 4]);
    });
});

describe('groupMarkersByLabel', () => {
    test('groups in alphabetical order, markers by time inside', () => {
        const groups = groupMarkersByLabel([
            m(1, 300, 'Kiss'),
            m(2, 100, 'Dance'),
            m(3, 50, 'Kiss'),
            m(4, 200, 'Dance'),
            m(5, 10, 'Bath'),
        ]);
        expect(groups.map((g) => g.label)).toEqual(['Bath', 'Dance', 'Kiss']);
        expect(groups.map((g) => ids(g.markers))).toEqual([[5], [2, 4], [3, 1]]);
    });

    test('unlabelled markers (empty or spaces) form the last group', () => {
        const groups = groupMarkersByLabel([
            m(1, 30, ''),
            m(2, 20, 'Zoom'),
            m(3, 10, '   '),
            m(4, 40, 'Alpha'),
        ]);
        expect(groups.map((g) => g.label)).toEqual(['Alpha', 'Zoom', null]);
        expect(ids(groups[2]!.markers)).toEqual([3, 1]);
        expect(groups[2]!.key).toBe('');
    });

    test('labels differing in case or spaces share a group named after its earliest marker', () => {
        const groups = groupMarkersByLabel([
            m(1, 90, 'kiss'),
            m(2, 10, ' Kiss '),
            m(3, 50, 'KISS'),
        ]);
        expect(groups).toHaveLength(1);
        expect(groups[0]!.label).toBe('Kiss');
        expect(ids(groups[0]!.markers)).toEqual([2, 3, 1]);
    });

    test('alphabetical order ignores case and orders numbers naturally', () => {
        const groups = groupMarkersByLabel([
            m(1, 0, 'b'),
            m(2, 0, 'A'),
            m(3, 0, 'Part 10'),
            m(4, 0, 'Part 2'),
        ]);
        expect(groups.map((g) => g.label)).toEqual(['A', 'b', 'Part 2', 'Part 10']);
    });

    test('a new marker lands in its group at its time', () => {
        const before = [m(1, 10, 'Kiss'), m(2, 100, 'Kiss'), m(3, 5, 'Bath')];
        const after = groupMarkersByLabel([...before, m(9, 50, 'kiss')]);
        expect(after.map((g) => ids(g.markers))).toEqual([[3], [1, 9, 2]]);
    });
});

describe('markerListGroups', () => {
    test('time order: one headerless group sorted by start', () => {
        const groups = markerListGroups([m(1, 30, 'B'), m(2, 10, ''), m(3, 20, 'A')], 'time');
        expect(groups).toHaveLength(1);
        expect(groups[0]!.label).toBeNull();
        expect(ids(groups[0]!.markers)).toEqual([2, 3, 1]);
    });
    test('label order: groups by label', () => {
        const groups = markerListGroups([m(1, 30, 'B'), m(2, 10, ''), m(3, 20, 'A')], 'label');
        expect(groups.map((g) => g.label)).toEqual(['A', 'B', null]);
    });
    test('no markers: no groups', () => {
        expect(markerListGroups([], 'time')).toEqual([]);
        expect(markerListGroups([], 'label')).toEqual([]);
    });
});
