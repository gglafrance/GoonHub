import { describe, expect, test } from 'bun:test';
import {
    defaultMarkerLabelMergeTarget,
    findMarkerLabel,
    markerLabelKey,
    pickMarkerLabel,
} from '../app/utils/markerLabels';

const labels = [
    { label: 'Kiss', count: 3 },
    { label: 'Blowjob', count: 1 },
];

describe('marker labels', () => {
    test('key ignores case and surrounding spaces', () => {
        expect(markerLabelKey('  KiSs ')).toBe('kiss');
    });

    test('finds a label ignoring case', () => {
        expect(findMarkerLabel(labels, ' kiss')?.label).toBe('Kiss');
        expect(findMarkerLabel(labels, 'hug')).toBeUndefined();
        expect(findMarkerLabel(labels, '   ')).toBeUndefined();
    });

    test('pick: empty, existing (canonical spelling), create or unknown', () => {
        expect(pickMarkerLabel('  ', labels, false)).toEqual({ kind: 'empty' });
        expect(pickMarkerLabel('BLOWJOB ', labels, false)).toEqual({
            kind: 'existing',
            name: 'Blowjob',
        });
        expect(pickMarkerLabel(' Hug ', labels, true)).toEqual({ kind: 'create', name: 'Hug' });
        expect(pickMarkerLabel('Hug', labels, false)).toEqual({ kind: 'unknown', name: 'Hug' });
        expect(pickMarkerLabel('x'.repeat(150), labels, true)).toEqual({
            kind: 'create',
            name: 'x'.repeat(100),
        });
    });

    test('merge target: most markers, then lowest id', () => {
        expect(
            defaultMarkerLabelMergeTarget([
                { id: 3, marker_count: 2 },
                { id: 2, marker_count: 5 },
                { id: 1, marker_count: 5 },
            ])?.id,
        ).toBe(1);
        expect(defaultMarkerLabelMergeTarget([])).toBeUndefined();
    });
});
