import type { Marker, UpdateMarkerRequest } from '~/types/marker';

/** New start and/or end of a marker. */
export type MarkerTimePatch = Pick<
    UpdateMarkerRequest,
    'timestamp' | 'end_timestamp' | 'clear_end_timestamp'
>;

export type MarkerTimeResult =
    | { ok: true; marker: Marker }
    | { ok: false; error: string }
    // A newer change of the same marker (or a scene change) came after this one: ignore it
    | { ok: false; stale: true };

interface Options {
    sceneId: () => number;
    // Optimistic update of the page's list, without waiting for a thumbnail (useNewMarkers)
    insertMarker: (marker: Marker) => void;
    // Update with the API response, waits for the new thumbnail when the start moved
    applyMarkerUpdate: (before: Marker, after: Marker) => void;
}

/**
 * Changes the start and/or end of a marker of the watch page: the marker moves at once
 * (timeline, Markers tab, scrubber), is updated with the API response, and is put back as it
 * was if the request fails. Shared by the Markers tab and the quick marker keys
 * (Shift+M, Alt+M).
 */
export const useMarkerTimeUpdate = ({ sceneId, insertMarker, applyMarkerUpdate }: Options) => {
    const { updateMarker } = useApiMarkers();
    // Latest request per marker: an older response arriving after it is ignored
    const latest = new Map<number, number>();
    let seq = 0;

    const update = async (
        marker: Marker,
        patch: MarkerTimePatch,
        fallbackError = 'Failed to update the marker',
    ): Promise<MarkerTimeResult> => {
        const scene = sceneId();
        const mine = ++seq;
        latest.set(marker.id, mine);
        const stale = () => latest.get(marker.id) !== mine || sceneId() !== scene;

        insertMarker({
            ...marker,
            timestamp: patch.timestamp ?? marker.timestamp,
            end_timestamp: patch.clear_end_timestamp
                ? null
                : (patch.end_timestamp ?? marker.end_timestamp),
        });
        try {
            const updated = await updateMarker(scene, marker.id, patch);
            if (stale()) return { ok: false, stale: true };
            latest.delete(marker.id);
            applyMarkerUpdate(marker, updated);
            return { ok: true, marker: updated };
        } catch (e) {
            if (stale()) return { ok: false, stale: true };
            latest.delete(marker.id);
            // Back to the marker as it was
            insertMarker(marker);
            return {
                ok: false,
                error: e instanceof Error && e.message ? e.message : fallbackError,
            };
        }
    };

    return { update };
};
