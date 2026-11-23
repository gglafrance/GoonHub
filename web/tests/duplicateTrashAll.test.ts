import { describe, expect, test } from 'bun:test';
import { describeBulkTrashAll, summarizeTrashAll } from '../app/utils/duplicateTrashAll';

const member = (scene_id: number, size: number, is_trashed = false) => ({
    scene_id,
    size,
    is_trashed,
});

describe('summarizeTrashAll', () => {
    const groups = new Map([
        [1, { members: [member(10, 100), member(11, 200)] }],
        [2, { members: [member(20, 50), member(21, 400, true), member(22, 0)] }],
    ]);

    test('counts every video not in the trash, the best one included, and sums sizes', () => {
        expect(summarizeTrashAll([1], groups)).toEqual({
            groups: 1,
            videos: 2,
            size: 300,
            unknownSize: 0,
            alreadyTrashed: 0,
            missingGroups: 0,
            unchecked: 0,
        });
    });

    test('leaves trashed videos out and counts unknown sizes apart', () => {
        expect(summarizeTrashAll([1, 2], groups)).toEqual({
            groups: 2,
            videos: 4,
            size: 350,
            unknownSize: 1,
            alreadyTrashed: 1,
            missingGroups: 0,
            unchecked: 0,
        });
    });

    test('counts groups not loaded as missing and each scene once', () => {
        const shared = new Map([
            [1, { members: [member(10, 100)] }],
            [3, { members: [member(10, 100)] }],
        ]);
        const summary = summarizeTrashAll([1, 3, 99], shared);
        expect(summary.videos).toBe(1);
        expect(summary.size).toBe(100);
        expect(summary.groups).toBe(2);
        expect(summary.missingGroups).toBe(1);
    });
});

describe('summarizeTrashAll with unchecked members', () => {
    test('leaves the unchecked members out and counts them apart', () => {
        const groups = new Map([
            [1, { members: [member(10, 100), member(11, 200), member(12, 50)] }],
        ]);
        const summary = summarizeTrashAll([1], groups, new Map([[1, new Set([11])]]));
        expect(summary.videos).toBe(2);
        expect(summary.size).toBe(150);
        expect(summary.unchecked).toBe(1);
    });
});

describe('describeBulkTrashAll', () => {
    test('describes resolved, skipped and failed groups', () => {
        expect(
            describeBulkTrashAll({
                resolved: [1, 2],
                failed: [{ group_id: 3, error: 'group is resolved' }],
                trashed_scenes: 5,
                skipped_scenes: 1,
            }),
        ).toBe(
            '2 group(s) resolved, 5 video(s) moved to the trash, 1 already in the trash or missing, 1 group(s) failed',
        );
    });

    test('omits empty parts', () => {
        expect(
            describeBulkTrashAll({
                resolved: [1],
                failed: [],
                trashed_scenes: 2,
                skipped_scenes: 0,
            }),
        ).toBe('1 group(s) resolved, 2 video(s) moved to the trash');
    });
});
