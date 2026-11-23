import { describe, expect, test } from 'bun:test';
import {
    eligibleMembers,
    planExclusions,
    pruneUnchecked,
    runExclusionPlan,
    setGroupUnchecked,
    toggleUnchecked,
    uncheckedMembers,
} from '../app/utils/duplicateEligibility';

const group = (id: number, ...sceneIds: number[]) => ({
    id,
    status: 'unresolved',
    members: sceneIds.map((scene_id) => ({ scene_id })),
});

describe('eligibility state', () => {
    test('every member is eligible by default; toggling unchecks then checks again', () => {
        const g = group(1, 10, 20, 30);
        let state = new Map<number, Set<number>>();
        expect(eligibleMembers(state, g)).toHaveLength(3);

        state = toggleUnchecked(state, 1, 20);
        expect(uncheckedMembers(state, g)).toEqual([20]);
        expect(eligibleMembers(state, g).map((m) => m.scene_id)).toEqual([10, 30]);

        const before = state;
        state = toggleUnchecked(state, 1, 20);
        expect(state.has(1)).toBe(false);
        expect(before.get(1)?.has(20)).toBe(true); // not mutated
    });

    test('setGroupUnchecked replaces a group, empty clears it', () => {
        let state = setGroupUnchecked(new Map(), 1, [10, 20]);
        expect([...state.get(1)!]).toEqual([10, 20]);
        state = setGroupUnchecked(state, 1, []);
        expect(state.size).toBe(0);
    });

    test('pruning drops scenes no longer members and closed groups', () => {
        const state = new Map([
            [1, new Set([10, 99])],
            [2, new Set([20])],
            [3, new Set([30])],
        ]);
        const pruned = pruneUnchecked(state, [
            group(1, 10, 11),
            { ...group(2, 20, 21), status: 'resolved' },
        ]);
        expect([...pruned.get(1)!]).toEqual([10]);
        expect(pruned.has(2)).toBe(false);
        expect(pruned.has(3)).toBe(true); // not reloaded: kept
    });
});

describe('planExclusions', () => {
    const groups = new Map([
        [1, group(1, 10, 11, 12)],
        [2, group(2, 20, 21)],
        [3, group(3, 30, 31)],
        [4, group(4, 40, 41)],
    ]);
    const state = new Map([
        [1, new Set([12])],
        [2, new Set([21])],
        [3, new Set([30, 31])],
    ]);

    test('lists the unchecked members per group, and groups with nothing eligible apart', () => {
        const plan = planExclusions([1, 2, 3, 4, 99], groups, state);
        expect(plan.exclusions).toEqual([
            { groupId: 1, sceneIds: [12], remaining: 2 },
            { groupId: 2, sceneIds: [21], remaining: 1 },
        ]);
        expect(plan.nothingEligible).toEqual([3]);
        expect(plan.uncheckedVideos).toBe(2);
    });

    test('runs the exclusions in order and reports the dismissed groups', async () => {
        const calls: [number, number[]][] = [];
        const dismissed = await runExclusionPlan(
            planExclusions([1, 2], groups, state),
            async (id, ids) => {
                calls.push([id, ids]);
                return { dismissed: id === 2 };
            },
        );
        expect(calls).toEqual([
            [1, [12]],
            [2, [21]],
        ]);
        expect([...dismissed]).toEqual([2]);
    });

    test('stops at the first failure', async () => {
        const calls: number[] = [];
        const run = runExclusionPlan(planExclusions([1, 2], groups, state), async (id) => {
            calls.push(id);
            throw new Error('boom');
        });
        await expect(run).rejects.toThrow('boom');
        expect(calls).toEqual([1]);
    });
});
