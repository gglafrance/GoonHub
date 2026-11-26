// Play queue of a playlist, whatever it holds: a scene playlist plays whole scenes, a marker
// playlist plays the range of each marker (docs/MARKER_PLAYLISTS.md). The watch page, its
// queue panels and the playlist page work on these items. Tests in web/tests/playlistQueue.test.ts.
import type { MarkerPlaylistFilter, PlaylistDetail, PlaylistKind } from '~/types/playlist';
import { markerThumbnailUrl } from './markerThumbnail';

export interface PlaylistQueueItem {
    /** Stable key: `scene-<id>` or `marker-<id>` */
    key: string;
    kind: PlaylistKind;
    sceneId: number;
    /** The marker played (marker playlists), else null */
    markerId: number | null;
    /** Scene title, or the marker label */
    title: string;
    /** Marker playlists: the scene title */
    subtitle: string;
    thumbnailUrl: string | null;
    /** Seconds played: the scene duration, or the marker range length */
    duration: number;
    /** Marker range played (seconds of the scene), null for a scene */
    start: number | null;
    end: number | null;
}

/** Number of items of a playlist (scenes or markers, by kind). */
export function playlistItemCount(detail: Pick<PlaylistDetail, 'kind' | 'scenes' | 'markers'>) {
    return detail.kind === 'markers' ? (detail.markers?.length ?? 0) : detail.scenes.length;
}

/** The play queue of a playlist, in playlist order. */
export function playlistQueueItems(
    detail: Pick<PlaylistDetail, 'kind' | 'scenes' | 'markers'>,
): PlaylistQueueItem[] {
    if (detail.kind === 'markers') {
        return (detail.markers ?? []).map((entry) => ({
            key: `marker-${entry.marker.id}`,
            kind: 'markers' as const,
            sceneId: entry.scene.id,
            markerId: entry.marker.id,
            title: entry.marker.label || 'Marker',
            subtitle: entry.scene.title,
            thumbnailUrl: entry.marker.has_thumbnail
                ? markerThumbnailUrl(entry.marker.id, entry.marker.thumbnail_version)
                : null,
            duration: Math.max(0, entry.end - entry.start),
            start: entry.start,
            end: entry.end,
        }));
    }
    return detail.scenes.map((entry) => ({
        key: `scene-${entry.scene.id}`,
        kind: 'scenes' as const,
        sceneId: entry.scene.id,
        markerId: null,
        title: entry.scene.title,
        subtitle: '',
        thumbnailUrl: entry.scene.thumbnail_path ? `/thumbnails/${entry.scene.id}` : null,
        duration: entry.scene.duration ?? 0,
        start: null,
        end: null,
    }));
}

/**
 * Query of the watch page playing an item: the playlist, the position in the play order, the
 * shuffle seed, and for a marker the time to start at (`t`, no `end`: the range plays once,
 * it is not looped).
 */
export function playlistItemQuery(
    uuid: string,
    item: Pick<PlaylistQueueItem, 'start'>,
    orderIndex: number,
    shuffleSeed = 0,
    startAt?: number,
): Record<string, string> {
    const query: Record<string, string> = { playlist: uuid, pos: String(orderIndex) };
    if (shuffleSeed) query.shuffle = String(shuffleSeed);
    if (item.start !== null) {
        query.t = String(Math.floor(startAt ?? item.start));
    } else if (startAt !== undefined && startAt > 0) {
        query.t = String(Math.floor(startAt));
    }
    return query;
}

/**
 * Whether playback just crossed the end of a marker range: from `previous` to `current`
 * seconds, by playing (a jump of more than `maxStep` seconds is a seek, which does not end the
 * marker, so the user can watch past it).
 */
export function crossedRangeEnd(
    end: number,
    previous: number,
    current: number,
    maxStep = 1.5,
): boolean {
    return previous < end && current >= end && current - previous <= maxStep;
}

/** Where to resume a playlist: its item index and the time to start at, or null. */
export function playlistResumeTarget(
    detail: Pick<PlaylistDetail, 'kind' | 'scenes' | 'markers' | 'resume'>,
): { index: number; item: PlaylistQueueItem; startAt: number } | null {
    const resume = detail.resume;
    if (!resume) return null;
    const items = playlistQueueItems(detail);
    if (detail.kind === 'markers') {
        if (!resume.marker_id) return null;
        const index = items.findIndex((i) => i.markerId === resume.marker_id);
        const item = items[index];
        if (!item || item.start === null || item.end === null) return null;
        // Inside the range: from there; else from the start of the marker
        const pos = resume.position_s;
        const startAt = pos >= item.start && pos < item.end - 1 ? pos : item.start;
        return { index, item, startAt };
    }
    if (!resume.scene_id) return null;
    const index = items.findIndex((i) => i.sceneId === resume.scene_id);
    const item = items[index];
    if (!item) return null;
    return { index, item, startAt: Math.max(0, resume.position_s) };
}

/**
 * Body of POST /playlists/:uuid/markers/from-filter for a marker listing: the query parameters
 * of the listing (useMarkerFilters params), its sort, and the label of a label page.
 */
export function markerPlaylistFilter(
    params: Record<string, string>,
    options: { label?: string; sort?: string } = {},
): MarkerPlaylistFilter {
    const filter: MarkerPlaylistFilter = {};
    if (options.label) filter.label = options.label;
    if (options.sort) filter.sort = options.sort;
    for (const key of ['labels', 'tags', 'actors', 'studio', 'marker_tags'] as const) {
        if (params[key]) filter[key] = params[key];
    }
    if (params.ranges_only === 'true') filter.ranges_only = true;
    const seed = Number(params.seed);
    if (params.seed && Number.isFinite(seed)) filter.seed = seed;
    return filter;
}
