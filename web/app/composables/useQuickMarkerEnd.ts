import type videojs from 'video.js';
import type { Marker } from '~/types/marker';

type Player = ReturnType<typeof videojs>;

interface Options {
    player: () => Player | null | undefined;
    sceneId: () => number;
    // Scene duration in seconds (the API refuses an end past it)
    duration: () => number | null | undefined;
    // Markers of the watch page (timeline, Markers tab, scrubber)
    markers: () => Marker[];
    // Last marker made with the quick marker during this visit of the scene
    lastQuickMarkerId: () => number | null;
    // Optimistic update of the page's list (useNewMarkers)
    applyMarkerUpdate: (before: Marker, after: Marker) => void;
    notify: (message: string, kind: 'success' | 'error') => void;
}

/**
 * Shift+M on the watch page: sets the end of the last quick marker (else of the marker with the
 * latest start before the current time) to the current player time, without pausing. Setting it
 * again moves the end. The marker updates at once, and is put back if the request fails.
 */
export const useQuickMarkerEnd = (options: Options) => {
    const { updateMarker } = useApiMarkers();
    // Latest request: an older response arriving after it is ignored
    let requestSeq = 0;

    const setEnd = async () => {
        const p = options.player();
        if (!p) return;
        // Same time source as M: currentTime() is the scene time (stream offset included)
        const end = quickMarkerEndTimestamp(p.currentTime(), options.duration());
        const target = pickQuickMarkerEndTarget(
            options.markers(),
            end,
            options.lastQuickMarkerId(),
        );
        const invalid = validateQuickMarkerEnd(target, end);
        if (invalid || !target) {
            options.notify(invalid ?? 'No marker before the current time', 'error');
            return;
        }

        const sceneId = options.sceneId();
        const seq = ++requestSeq;
        // Stale once a newer request started or the scene changed (another markers list)
        const stale = () => seq !== requestSeq || options.sceneId() !== sceneId;
        options.applyMarkerUpdate(target, { ...target, end_timestamp: end });
        try {
            const updated = await updateMarker(sceneId, target.id, { end_timestamp: end });
            if (stale()) return;
            options.applyMarkerUpdate(target, updated);
            options.notify(
                quickMarkerEndToastMessage(target.timestamp, end, target.label),
                'success',
            );
        } catch (e) {
            if (stale()) return;
            // Back to the marker as it was
            options.applyMarkerUpdate(target, target);
            options.notify(
                e instanceof Error && e.message ? e.message : 'Failed to set the marker end',
                'error',
            );
        }
    };

    return { setEnd };
};
