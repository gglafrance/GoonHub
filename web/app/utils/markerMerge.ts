// Pure helpers telling what happened to markers when duplicates were merged, and what the
// marker deduplication maintenance found. Unit tested in web/tests/markerMerge.test.ts.
import type { MarkerMergeSummary } from '~/types/duplicates';
import type { MarkerDedupMerge, MarkerDedupResult } from '~/types/marker';
import { formatMarkerTime } from './markerTime';

function count(n: number, one: string, many: string): string {
    return `${n} ${n === 1 ? one : many}`;
}

/**
 * What a resolution did with the markers of the trashed scenes, e.g. "3 markers moved
 * (2 shifted), 1 duplicate removed, 1 left on the trashed scene". Empty when there were none.
 */
export function describeMarkerMerge(summary: MarkerMergeSummary | null | undefined): string {
    if (!summary) return '';
    const parts: string[] = [];
    if (summary.moved > 0) {
        parts.push(
            count(summary.moved, 'marker moved', 'markers moved') +
                (summary.shifted > 0 ? ` (${summary.shifted} shifted)` : ''),
        );
    }
    if (summary.deduplicated > 0) {
        parts.push(count(summary.deduplicated, 'duplicate removed', 'duplicates removed'));
    }
    if (summary.left_behind > 0) {
        parts.push(
            count(summary.left_behind, 'marker', 'markers') +
                ' left on the trashed scene (outside the matching part)',
        );
    }
    return parts.join(', ');
}

/** Short result of a marker deduplication run (dry run or applied). */
export function describeMarkerDedup(result: MarkerDedupResult): string {
    if (result.removed === 0) {
        return `No duplicate markers found (${count(result.scenes_scanned, 'scene', 'scenes')} checked)`;
    }
    const where = `in ${count(result.scenes, 'scene', 'scenes')}`;
    const markers = count(result.removed, 'duplicate marker', 'duplicate markers');
    return result.dry_run ? `${markers} ${where} would be removed` : `${markers} ${where} removed`;
}

/** One folded set: "Kiss at 01:20: keeps #12, removes #13, #14". */
export function describeMarkerDedupMerge(merge: MarkerDedupMerge): string {
    const label = merge.label || 'Unlabelled';
    const removed = merge.removed.map((m) => `#${m.id}`).join(', ');
    return `${label} at ${formatMarkerTime(merge.kept.timestamp)}: keeps #${merge.kept.id}, removes ${removed}`;
}
