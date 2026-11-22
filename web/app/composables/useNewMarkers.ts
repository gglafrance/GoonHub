import type { Marker } from '~/types/marker';

// Marker thumbnails are made in the background after creation, and remade (same file name, new
// version) when a marker moves: poll the markers until they have theirs (or give up after a
// while, the marker works without it)
const THUMBNAIL_POLL_INTERVAL_MS = 2000;
const THUMBNAIL_POLL_MAX_ATTEMPTS = 20;

interface Options {
    // List of the watch page (timeline and Markers tab)
    markers: Ref<Marker[]> | undefined;
    refresh: (() => Promise<void>) | undefined;
    // Whether a marker has the thumbnail that is waited for (default: either type, the
    // server only makes the one it is configured for)
    hasThumbnail?: (marker: Marker) => boolean;
}

/**
 * Shows markers created or edited on the watch page at once (optimistic update of the page's
 * list), then refetches the list until their thumbnails exist (or, for a moved marker, until
 * the new one is written). Shared by the Markers tab and the quick marker.
 */
export const useNewMarkers = ({ markers, refresh, hasThumbnail }: Options) => {
    // Markers waited for, with the thumbnail version to move past (null: any thumbnail)
    const pendingThumbnails = new Map<number, string | null>();
    // Thumbnail version shown while a moved marker's new thumbnail is made: the server deletes
    // the old file at once, so the new version has no image until then (a broken image), while
    // the browser still has the old version cached
    const heldVersions = new Map<number, string>();
    let thumbnailPollTimer: ReturnType<typeof setTimeout> | null = null;
    let thumbnailPollAttempts = 0;
    let unmounted = false;

    const thumbnailReady = (marker: Marker) =>
        hasThumbnail
            ? hasThumbnail(marker)
            : !!(marker.thumbnail_path || marker.animated_thumbnail_path);

    // Adds (or replaces) a marker in the page's list, kept in timestamp order like the API
    // returns it
    const insertMarker = (marker: Marker) => {
        if (!markers) return;
        const list = markers.value.filter((m) => m.id !== marker.id);
        list.push(marker);
        list.sort((a, b) => a.timestamp - b.timestamp || a.id - b.id);
        markers.value = list;
    };

    const pollThumbnails = async () => {
        thumbnailPollTimer = null;
        await refresh?.();
        if (unmounted) return;
        const held = new Map<number, string>();
        for (const [id, since] of [...pendingThumbnails]) {
            const marker = markers?.value.find((m) => m.id === id);
            if (
                !marker ||
                (thumbnailReady(marker) &&
                    (since == null || markerThumbnailVersionChanged(since, marker.updated_at)))
            ) {
                pendingThumbnails.delete(id);
                heldVersions.delete(id);
            } else if (heldVersions.has(id)) {
                held.set(id, heldVersions.get(id)!);
            }
        }
        if (markers && held.size) {
            markers.value = markers.value.map((m) =>
                held.has(m.id) ? { ...m, updated_at: held.get(m.id)! } : m,
            );
        }
        thumbnailPollAttempts++;
        if (pendingThumbnails.size === 0 || thumbnailPollAttempts >= THUMBNAIL_POLL_MAX_ATTEMPTS) {
            pendingThumbnails.clear();
            heldVersions.clear();
            return;
        }
        thumbnailPollTimer = setTimeout(pollThumbnails, THUMBNAIL_POLL_INTERVAL_MS);
    };

    // Refetches until the marker has a thumbnail; with `since` (the marker's updated_at once
    // moved), until a thumbnail newer than that is written
    const waitForThumbnail = (markerId: number, since: string | null = null) => {
        pendingThumbnails.set(markerId, since);
        thumbnailPollAttempts = 0;
        if (!thumbnailPollTimer) {
            thumbnailPollTimer = setTimeout(pollThumbnails, THUMBNAIL_POLL_INTERVAL_MS);
        }
    };

    // Shows an edited marker at once (the PUT response has no tags: the listed ones are kept),
    // and waits for its new thumbnail when it moved (the server remakes it in the background),
    // showing the previous one meanwhile
    const applyMarkerUpdate = (before: Marker, after: Marker) => {
        const moved = after.timestamp !== before.timestamp;
        if (moved && !heldVersions.has(after.id)) heldVersions.set(after.id, before.updated_at);
        const held = heldVersions.get(after.id);
        insertMarker({
            ...before,
            ...after,
            tags: after.tags ?? before.tags,
            ...(held ? { updated_at: held } : {}),
        });
        if (moved) waitForThumbnail(after.id, after.updated_at);
    };

    onBeforeUnmount(() => {
        unmounted = true;
        if (thumbnailPollTimer) clearTimeout(thumbnailPollTimer);
        thumbnailPollTimer = null;
        pendingThumbnails.clear();
        heldVersions.clear();
    });

    return { insertMarker, waitForThumbnail, applyMarkerUpdate };
};
