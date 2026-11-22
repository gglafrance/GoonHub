import { describe, expect, test } from 'bun:test';
import { toggleSceneInSelection } from '../app/utils/sceneSelection';

const ids = [10, 20, 30, 40, 50];
const sorted = (set: Set<number>) => [...set].sort((a, b) => a - b);

describe('toggleSceneInSelection', () => {
    test('a plain click selects an unselected scene', () => {
        expect(sorted(toggleSceneInSelection(new Set(), ids, 30, null, false))).toEqual([30]);
    });

    test('a plain click deselects a selected scene', () => {
        expect(sorted(toggleSceneInSelection(new Set([20, 30]), ids, 30, 20, false))).toEqual([20]);
    });

    test('a plain click ignores the anchor', () => {
        expect(sorted(toggleSceneInSelection(new Set([10]), ids, 40, 10, false))).toEqual([10, 40]);
    });

    test('a shift-click selects the range forward from the anchor', () => {
        expect(sorted(toggleSceneInSelection(new Set([20]), ids, 40, 20, true))).toEqual([
            20, 30, 40,
        ]);
    });

    test('a shift-click selects the range backward from the anchor', () => {
        expect(sorted(toggleSceneInSelection(new Set([50]), ids, 20, 50, true))).toEqual([
            20, 30, 40, 50,
        ]);
    });

    test('a shift-click keeps the selected scenes outside the range', () => {
        expect(sorted(toggleSceneInSelection(new Set([10, 50]), ids, 40, 30, true))).toEqual([
            10, 30, 40, 50,
        ]);
    });

    test('a shift-click on a selected scene deselects the range', () => {
        const selected = new Set([10, 20, 30, 40]);
        expect(sorted(toggleSceneInSelection(selected, ids, 20, 40, true))).toEqual([10]);
    });

    test('a shift-click without an anchor toggles the scene only', () => {
        expect(sorted(toggleSceneInSelection(new Set(), ids, 30, null, true))).toEqual([30]);
    });

    test('a shift-click with an anchor no longer listed toggles the scene only', () => {
        expect(sorted(toggleSceneInSelection(new Set([99]), ids, 30, 99, true))).toEqual([30, 99]);
    });

    test('a shift-click on the anchor itself toggles it', () => {
        expect(sorted(toggleSceneInSelection(new Set([30]), ids, 30, 30, true))).toEqual([]);
    });

    test('the input set is not modified', () => {
        const selected = new Set([20]);
        toggleSceneInSelection(selected, ids, 40, 20, true);
        expect(sorted(selected)).toEqual([20]);
    });
});
