import type { Marker } from '~/types/marker';

/** Order of the watch page markers list: by start time, or grouped by label. */
export type MarkerListSort = 'time' | 'label';

export const MARKER_LIST_SORT_STORAGE_KEY = 'watch-markers-sort';

export const MARKER_LIST_SORT_OPTIONS: { value: MarkerListSort; label: string }[] = [
    { value: 'time', label: 'By time' },
    { value: 'label', label: 'By label' },
];

type ListMarker = Pick<Marker, 'id' | 'timestamp' | 'label'>;

export interface MarkerGroup<T extends ListMarker> {
    /** Stable key (lowercased label, '' for the unlabelled group, 'all' for the time view). */
    key: string;
    /** Header text; null for the unlabelled group and the single group of the time view. */
    label: string | null;
    markers: T[];
}

/** Reads a stored sort value, falling back to the time order. */
export function parseMarkerListSort(value: string | null | undefined): MarkerListSort {
    return value === 'label' ? 'label' : 'time';
}

/** Tab label: "Markers (3)", plain "Markers" when the scene has none. */
export function markerTabLabel(count: number): string {
    return count > 0 ? `Markers (${count})` : 'Markers';
}

/** Markers by start time (ties by id, as the API and useNewMarkers keep them); copies the list. */
export function sortMarkersByTime<T extends ListMarker>(markers: readonly T[]): T[] {
    return [...markers].sort((a, b) => a.timestamp - b.timestamp || a.id - b.id);
}

/**
 * Markers grouped by label (case and surrounding spaces ignored), groups in alphabetical
 * order, markers by time inside a group, unlabelled markers in a last group. A group header
 * shows the label of its earliest marker.
 */
export function groupMarkersByLabel<T extends ListMarker>(markers: readonly T[]): MarkerGroup<T>[] {
    const groups = new Map<string, MarkerGroup<T>>();
    for (const marker of sortMarkersByTime(markers)) {
        const label = marker.label?.trim() ?? '';
        const key = label.toLocaleLowerCase();
        let group = groups.get(key);
        if (!group) {
            group = { key, label: label || null, markers: [] };
            groups.set(key, group);
        }
        group.markers.push(marker);
    }
    const unlabelled = groups.get('');
    groups.delete('');
    const sorted = [...groups.values()].sort((a, b) =>
        (a.label ?? '').localeCompare(b.label ?? '', undefined, {
            sensitivity: 'base',
            numeric: true,
        }),
    );
    if (unlabelled) sorted.push(unlabelled);
    return sorted;
}

/** Groups to render for the chosen order: a single headerless group for the time order. */
export function markerListGroups<T extends ListMarker>(
    markers: readonly T[],
    sort: MarkerListSort,
): MarkerGroup<T>[] {
    if (markers.length === 0) return [];
    if (sort === 'label') return groupMarkersByLabel(markers);
    return [{ key: 'all', label: null, markers: sortMarkersByTime(markers) }];
}
