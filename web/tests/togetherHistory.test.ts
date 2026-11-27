import { describe, expect, test } from 'bun:test';
import type { HistoryEntry } from '../app/types/together';
import {
    MAX_ITEMS_PER_ADD,
    historyEntryToQueueItem,
    itemRangeText,
    pushHistoryEntry,
    requeueAllItems,
} from '../app/utils/togetherHistory';

const entry = (id: number, over: Partial<HistoryEntry> = {}): HistoryEntry => ({
    id,
    scene_id: id,
    title: `Scene ${id}`,
    duration: 120,
    start: null,
    end: null,
    label: null,
    added_by: { id: 1, username: 'alice' },
    started_at: 1000 * id,
    ended_at: 1000 * id + 500,
    outcome: 'completed',
    available: true,
    ...over,
});

describe('history list', () => {
    test('a new entry goes first, bounded, without duplicates', () => {
        let list: HistoryEntry[] = [];
        list = pushHistoryEntry(list, entry(1), 3);
        list = pushHistoryEntry(list, entry(2), 3);
        list = pushHistoryEntry(list, entry(3), 3);
        expect(list.map((e) => e.id)).toEqual([3, 2, 1]);
        list = pushHistoryEntry(list, entry(4), 3);
        expect(list.map((e) => e.id)).toEqual([4, 3, 2]);
        // the same entry again (a resent message) is not duplicated
        list = pushHistoryEntry(list, entry(4, { outcome: 'skipped' }), 3);
        expect(list.map((e) => e.id)).toEqual([4, 3, 2]);
        expect(list[0]?.outcome).toBe('skipped');
    });

    test('no bound when the size is unknown', () => {
        const list = [entry(2), entry(1)];
        expect(pushHistoryEntry(list, entry(3), 0)).toHaveLength(3);
    });
});

describe('requeue', () => {
    test('an entry is queued again with its scene, range and label', () => {
        const marker = entry(5, { start: 30, end: 60, label: 'Intro' });
        expect(historyEntryToQueueItem(marker)).toEqual({
            scene_id: 5,
            start: 30,
            end: 60,
            label: 'Intro',
        });
        expect(historyEntryToQueueItem(entry(6))).toEqual({
            scene_id: 6,
            start: null,
            end: null,
            label: null,
        });
    });

    test('requeue all: oldest first, available only, each item once', () => {
        // newest first, as the room keeps it
        const list = [
            entry(13, { scene_id: 1 }), // scene 1 played again: queued once
            entry(12, { scene_id: 2, available: false }), // trashed since
            entry(11, { scene_id: 1, start: 10, end: 20, label: 'Marker' }), // another item
            entry(10, { scene_id: 3 }),
            entry(9, { scene_id: 1 }),
        ];
        expect(requeueAllItems(list)).toEqual([
            { scene_id: 1, start: null, end: null, label: null },
            { scene_id: 3, start: null, end: null, label: null },
            { scene_id: 1, start: 10, end: 20, label: 'Marker' },
        ]);
        expect(requeueAllItems([])).toEqual([]);
    });

    test('requeue all stays within one queue_add', () => {
        const list = Array.from({ length: MAX_ITEMS_PER_ADD + 20 }, (_, i) => entry(i + 1));
        const items = requeueAllItems(list);
        expect(items).toHaveLength(MAX_ITEMS_PER_ADD);
        // the oldest ones come first
        expect(items[0]?.scene_id).toBe(MAX_ITEMS_PER_ADD + 20);
    });
});

describe('item range', () => {
    test('range, open range, duration, nothing', () => {
        expect(itemRangeText({ start: 30, end: 90, duration: 600 })).toBe('0:30 - 1:30');
        expect(itemRangeText({ start: 30, end: null, duration: 600 })).toBe('0:30 - 10:00');
        expect(itemRangeText({ start: null, end: null, duration: 3725 })).toBe('1:02:05');
        expect(itemRangeText({ start: null, end: null, duration: 0 })).toBe('');
    });
});
