import type { Marker } from '~/types/marker';

// Marker thumbnails are made in the background after creation: poll the markers until the
// new ones have theirs (or give up after a while, the marker works without it)
const THUMBNAIL_POLL_INTERVAL_MS = 2000;
const THUMBNAIL_POLL_MAX_ATTEMPTS = 20;

interface Options {
    // List of the watch page (timeline and Markers tab)
    markers: Ref<Marker[]> | undefined;
    refresh: (() => Promise<void>) | undefined;
    // Whether a marker has the thumbnail that is waited for (default: the static one)
    hasThumbnail?: (marker: Marker) => boolean;
}

/**
 * Shows markers created on the watch page at once (optimistic insert into the page's list),
 * then refetches the list until their thumbnails exist. Shared by the Markers tab and the
 * quick marker.
 */
export const useNewMarkers = ({ markers, refresh, hasThumbnail }: Options) => {
    const pendingThumbnailIds = new Set<number>();
    let thumbnailPollTimer: ReturnType<typeof setTimeout> | null = null;
    let thumbnailPollAttempts = 0;
    let unmounted = false;

    const thumbnailReady = (marker: Marker) =>
        hasThumbnail ? hasThumbnail(marker) : !!marker.thumbnail_path;

    // Adds a marker to the page's list, kept in timestamp order like the API returns it
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
        for (const id of [...pendingThumbnailIds]) {
            const marker = markers?.value.find((m) => m.id === id);
            if (!marker || thumbnailReady(marker)) pendingThumbnailIds.delete(id);
        }
        thumbnailPollAttempts++;
        if (
            pendingThumbnailIds.size === 0 ||
            thumbnailPollAttempts >= THUMBNAIL_POLL_MAX_ATTEMPTS
        ) {
            pendingThumbnailIds.clear();
            return;
        }
        thumbnailPollTimer = setTimeout(pollThumbnails, THUMBNAIL_POLL_INTERVAL_MS);
    };

    const waitForThumbnail = (markerId: number) => {
        pendingThumbnailIds.add(markerId);
        thumbnailPollAttempts = 0;
        if (!thumbnailPollTimer) {
            thumbnailPollTimer = setTimeout(pollThumbnails, THUMBNAIL_POLL_INTERVAL_MS);
        }
    };

    onBeforeUnmount(() => {
        unmounted = true;
        if (thumbnailPollTimer) clearTimeout(thumbnailPollTimer);
        thumbnailPollTimer = null;
        pendingThumbnailIds.clear();
    });

    return { insertMarker, waitForThumbnail };
};
