import type { Marker } from '~/types/marker';

type MarkerRange = Pick<Marker, 'timestamp' | 'end_timestamp'>;

/** True when the marker covers a range (has an end after its start). */
export function markerHasRange(
    marker: MarkerRange,
): marker is MarkerRange & { end_timestamp: number } {
    return marker.end_timestamp != null && marker.end_timestamp > marker.timestamp;
}

/** Watch page URL that starts playback at the marker (and passes its end for range looping). */
export function markerWatchUrl(marker: MarkerRange & Pick<Marker, 'scene_id'>): string {
    const base = `/watch/${marker.scene_id}?t=${marker.timestamp}`;
    return markerHasRange(marker) ? `${base}&end=${marker.end_timestamp}` : base;
}
