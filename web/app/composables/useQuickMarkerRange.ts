import type videojs from 'video.js';
import type { Marker } from '~/types/marker';

type Player = ReturnType<typeof videojs>;

interface Options {
    player: () => Player | null | undefined;
    sceneId: () => number;
    // Scene duration in seconds (the API refuses a time past it)
    duration: () => number | null | undefined;
    // Markers of the watch page (timeline, Markers tab, scrubber)
    markers: () => Marker[];
    // Last marker made with the quick marker during this visit of the scene
    lastQuickMarkerId: () => number | null;
    // Optimistic update of the page's list (useNewMarkers)
    insertMarker: (marker: Marker) => void;
    applyMarkerUpdate: (before: Marker, after: Marker) => void;
    notify: (message: string, kind: 'success' | 'error') => void;
}

/**
 * Shift+M and Alt+M on the watch page: set the end (Shift+M) or the start (Alt+M) of the last
 * quick marker (else of the marker with the latest start before the current time) to the
 * current player time, without pausing. Setting it again moves it. The marker updates at once,
 * and is put back if the request fails.
 */
export const useQuickMarkerRange = (options: Options) => {
    const { update } = useMarkerTimeUpdate({
        sceneId: options.sceneId,
        insertMarker: options.insertMarker,
        applyMarkerUpdate: options.applyMarkerUpdate,
    });

    // Same time source as M: currentTime() is the scene time (stream offset included), kept
    // within the scene duration
    const playerTimestamp = (): number | null => {
        const p = options.player();
        return p ? quickMarkerEndTimestamp(p.currentTime(), options.duration()) : null;
    };

    const setEnd = async () => {
        const end = playerTimestamp();
        if (end == null) return;
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

        const result = await update(target, { end_timestamp: end }, 'Failed to set the marker end');
        if (result.ok) {
            options.notify(
                quickMarkerEndToastMessage(target.timestamp, end, target.label),
                'success',
            );
        } else if (!('stale' in result)) {
            options.notify(result.error, 'error');
        }
    };

    const setStart = async () => {
        const start = playerTimestamp();
        if (start == null) return;
        const target = pickQuickMarkerStartTarget(
            options.markers(),
            start,
            options.lastQuickMarkerId(),
        );
        const invalid = validateQuickMarkerStart(target, start);
        if (invalid || !target) {
            options.notify(invalid ?? 'No marker before the current time', 'error');
            return;
        }

        const result = await update(target, { timestamp: start }, 'Failed to set the marker start');
        if (result.ok) {
            options.notify(
                quickMarkerStartToastMessage(start, result.marker.end_timestamp, target.label),
                'success',
            );
        } else if (!('stale' in result)) {
            options.notify(result.error, 'error');
        }
    };

    return { setStart, setEnd };
};
