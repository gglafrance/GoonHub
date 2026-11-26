// Goon2gether history of the played items (docs/GOON2GETHER.md "History"): the client list
// kept from the welcome snapshot and the history_add messages, and the items to requeue.
// Tests in web/tests/togetherHistory.test.ts.
import type { HistoryEntry, QueueItem, QueueItemInput } from '~/types/together';
import { formatClock } from './together';

/** Most items of one queue_add (the server refuses more). */
export const MAX_ITEMS_PER_ADD = 100;

/**
 * Adds the newest entry in front of the list (newest first), without a duplicate id, keeping
 * at most `max` entries (`max` <= 0: no bound).
 */
export function pushHistoryEntry(
    list: readonly HistoryEntry[],
    entry: HistoryEntry,
    max: number,
): HistoryEntry[] {
    const next = [entry, ...list.filter((e) => e.id !== entry.id)];
    return max > 0 && next.length > max ? next.slice(0, max) : next;
}

/** The queue item to add again for a history entry: same scene, range and label. */
export function historyEntryToQueueItem(entry: HistoryEntry): QueueItemInput {
    return { scene_id: entry.scene_id, start: entry.start, end: entry.end, label: entry.label };
}

const itemKey = (i: QueueItemInput) =>
    `${i.scene_id}|${i.start ?? ''}|${i.end ?? ''}|${i.label ?? ''}`;

/**
 * "Requeue all": the available entries in the order they played (oldest first), each item
 * once (an item played several times, e.g. with repeat_queue, is queued once), at most
 * MAX_ITEMS_PER_ADD items.
 */
export function requeueAllItems(entries: readonly HistoryEntry[]): QueueItemInput[] {
    const seen = new Set<string>();
    const out: QueueItemInput[] = [];
    for (let i = entries.length - 1; i >= 0 && out.length < MAX_ITEMS_PER_ADD; i--) {
        const entry = entries[i];
        if (!entry?.available) continue;
        const item = historyEntryToQueueItem(entry);
        const key = itemKey(item);
        if (seen.has(key)) continue;
        seen.add(key);
        out.push(item);
    }
    return out;
}

/** Range of a queue item or history entry ("1:00 - 1:30"), else its duration, else ''. */
export function itemRangeText(item: Pick<QueueItem, 'start' | 'end' | 'duration'>): string {
    const { start, end, duration } = item;
    if (start != null || end != null) {
        return `${formatClock(start ?? 0)} - ${formatClock(end ?? duration)}`;
    }
    return duration > 0 ? formatClock(duration) : '';
}
