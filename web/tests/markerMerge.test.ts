import { describe, expect, test } from 'bun:test';
import type { MarkerDedupResult } from '../app/types/marker';
import {
    describeMarkerDedup,
    describeMarkerDedupMerge,
    describeMarkerMerge,
} from '../app/utils/markerMerge';

const result = (over: Partial<MarkerDedupResult> = {}): MarkerDedupResult => ({
    dry_run: true,
    scenes_scanned: 4,
    scenes: 2,
    groups: 2,
    removed: 3,
    merges: [],
    truncated: false,
    ...over,
});

describe('describeMarkerMerge', () => {
    test('nothing to say without markers', () => {
        expect(describeMarkerMerge(undefined)).toBe('');
        expect(describeMarkerMerge({ moved: 0, shifted: 0, deduplicated: 0, left_behind: 0 })).toBe(
            '',
        );
    });

    test('moved, shifted, deduplicated and left behind', () => {
        expect(describeMarkerMerge({ moved: 3, shifted: 2, deduplicated: 1, left_behind: 2 })).toBe(
            '3 markers moved (2 shifted), 1 duplicate removed, ' +
                '2 markers left on the trashed scene (outside the matching part)',
        );
    });

    test('singular and no shift', () => {
        expect(describeMarkerMerge({ moved: 1, shifted: 0, deduplicated: 2, left_behind: 0 })).toBe(
            '1 marker moved, 2 duplicates removed',
        );
        expect(describeMarkerMerge({ moved: 0, shifted: 0, deduplicated: 0, left_behind: 1 })).toBe(
            '1 marker left on the trashed scene (outside the matching part)',
        );
    });
});

describe('describeMarkerDedup', () => {
    test('nothing found', () => {
        expect(describeMarkerDedup(result({ removed: 0, scenes: 0, groups: 0 }))).toBe(
            'No duplicate markers found (4 scenes checked)',
        );
        expect(describeMarkerDedup(result({ removed: 0, scenes_scanned: 1 }))).toBe(
            'No duplicate markers found (1 scene checked)',
        );
    });

    test('dry run and applied', () => {
        expect(describeMarkerDedup(result())).toBe(
            '3 duplicate markers in 2 scenes would be removed',
        );
        expect(describeMarkerDedup(result({ dry_run: false, removed: 1, scenes: 1 }))).toBe(
            '1 duplicate marker in 1 scene removed',
        );
    });
});

describe('describeMarkerDedupMerge', () => {
    test('label, time and markers', () => {
        expect(
            describeMarkerDedupMerge({
                scene_id: 7,
                user_id: 1,
                kept: { id: 12, timestamp: 80, label: 'Kiss' },
                label: 'Kiss',
                removed: [
                    { id: 13, timestamp: 81, label: 'kiss' },
                    { id: 14, timestamp: 79, label: '' },
                ],
                added_tag_ids: [],
            }),
        ).toBe('Kiss at 01:20: keeps #12, removes #13, #14');
    });

    test('unlabelled set', () => {
        expect(
            describeMarkerDedupMerge({
                scene_id: 7,
                user_id: 1,
                kept: { id: 1, timestamp: 5, label: '' },
                label: '',
                removed: [{ id: 2, timestamp: 6, label: '' }],
                added_tag_ids: [],
            }),
        ).toBe('Unlabelled at 00:05: keeps #1, removes #2');
    });
});
