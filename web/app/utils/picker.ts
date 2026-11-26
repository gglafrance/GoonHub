// Search picker modal (components/picker): what can be picked, the selection rules and the
// mapping of picked items to Goon2gether queue items. Pure functions, tested in
// tests/picker.test.ts.
import type { SceneListItem } from '~/types/scene';
import type { MarkerWithScene } from '~/types/marker';
import type { QueueItemInput } from '~/types/together';
import { markerThumbnailUrl } from './markerThumbnail';

export type PickerTab = 'scenes' | 'markers';

export type PickedItem =
    | { kind: 'scene'; scene: SceneListItem }
    | { kind: 'marker'; marker: MarkerWithScene };

/** An extra button of the picker footer (e.g. "Play now"), next to the confirm button. */
export interface PickerAction {
    id: string;
    label: string;
    icon?: string;
}

/** Ids that are already picked by the caller (shown as such, see PickerOptions.excludedSelectable). */
export interface PickerExclude {
    scenes?: readonly number[];
    markers?: readonly number[];
}

export interface PickerOptions {
    /** Several items (else one: a click replaces the selection) */
    multiple: boolean;
    /** Most items in the selection (multi-select), 0 or less = no limit */
    max?: number;
    /** Keys of the items already picked (pickerKey) */
    excluded?: ReadonlySet<string>;
    /** Excluded items may be picked again (marked only, e.g. a scene already in the queue) */
    excludedSelectable?: boolean;
}

/** Stable key of a picked item: "scene:12", "marker:5". */
export function pickerKey(item: PickedItem): string {
    return item.kind === 'scene' ? `scene:${item.scene.id}` : `marker:${item.marker.id}`;
}

export const sceneKey = (id: number) => `scene:${id}`;
export const markerKey = (id: number) => `marker:${id}`;

/** The keys of the excluded ids. */
export function excludedKeys(exclude: PickerExclude | undefined): Set<string> {
    const out = new Set<string>();
    for (const id of exclude?.scenes ?? []) out.add(sceneKey(id));
    for (const id of exclude?.markers ?? []) out.add(markerKey(id));
    return out;
}

/** Whether the item can be added to the selection now (not the case of a selected item). */
export function canPick(
    selection: readonly PickedItem[],
    item: PickedItem,
    opts: PickerOptions,
): boolean {
    if (!opts.excludedSelectable && opts.excluded?.has(pickerKey(item))) return false;
    if (!opts.multiple) return true;
    const max = opts.max ?? 0;
    return max <= 0 || selection.length < max;
}

/**
 * Toggles an item: a selected item is removed; else it is added (multi-select, while under the
 * limit) or replaces the selection (single select). An item that cannot be picked leaves the
 * selection unchanged (same array).
 */
export function togglePick(
    selection: readonly PickedItem[],
    item: PickedItem,
    opts: PickerOptions,
): PickedItem[] {
    const key = pickerKey(item);
    if (selection.some((i) => pickerKey(i) === key)) {
        return selection.filter((i) => pickerKey(i) !== key);
    }
    if (!canPick(selection, item, opts)) return selection as PickedItem[];
    return opts.multiple ? [...selection, item] : [item];
}

/** Seconds a marker without an end plays (server default `playlists.marker_default_duration`). */
export const MARKER_DEFAULT_PLAY_SECONDS = 30;

/**
 * The queue item of a marker: its range (a marker without an end plays `defaultDuration`
 * seconds; the server cuts the range at the end of the scene) and its label.
 */
export function markerQueueItem(
    marker: Pick<MarkerWithScene, 'scene_id' | 'timestamp' | 'end_timestamp' | 'label'>,
    defaultDuration = MARKER_DEFAULT_PLAY_SECONDS,
): QueueItemInput {
    const start = Math.max(0, marker.timestamp);
    const end =
        marker.end_timestamp != null && marker.end_timestamp > start
            ? marker.end_timestamp
            : start + defaultDuration;
    const label = marker.label?.trim();
    return { scene_id: marker.scene_id, start, end, label: label || null };
}

/** Queue items of the picked items, in the selection order (scenes whole, markers as ranges). */
export function pickedToQueueItems(
    items: readonly PickedItem[],
    defaultDuration = MARKER_DEFAULT_PLAY_SECONDS,
): QueueItemInput[] {
    return items.map((i) =>
        i.kind === 'scene' ? { scene_id: i.scene.id } : markerQueueItem(i.marker, defaultDuration),
    );
}

/** The picked scenes, in order. */
export function pickedScenes(items: readonly PickedItem[]): SceneListItem[] {
    return items.flatMap((i) => (i.kind === 'scene' ? [i.scene] : []));
}

/** The picked markers, in order. */
export function pickedMarkers(items: readonly PickedItem[]): MarkerWithScene[] {
    return items.flatMap((i) => (i.kind === 'marker' ? [i.marker] : []));
}

/** Thumbnail and title of a picked item (selection tray). */
export function pickedSummary(item: PickedItem): { title: string; thumb: string | null } {
    if (item.kind === 'scene') {
        const s = item.scene;
        return {
            title: s.title || `Scene ${s.id}`,
            thumb: s.thumbnail_path ? `/thumbnails/${s.id}?size=sm` : null,
        };
    }
    const m = item.marker;
    return {
        title: `${m.label || 'Marker'} - ${m.scene_title}`,
        thumb: markerThumbnailUrl(m.id, m.updated_at),
    };
}

/** Text of the confirm button: "Add", "Add 3". */
export function confirmText(label: string, count: number, multiple: boolean): string {
    return multiple && count > 0 ? `${label} ${count}` : label;
}
