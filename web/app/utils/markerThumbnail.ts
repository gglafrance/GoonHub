// URLs of the marker thumbnails (static webp, animated mp4 clip). A marker's thumbnail is
// rewritten under the same name when the marker moves, so the URLs carry a version (?v=): the
// marker's updated_at in Unix milliseconds, bumped by the server each time a thumbnail is
// written (data.MarkerThumbnailVersion). A new version makes the browser fetch the new file;
// an unchanged one stays cached. Tests in web/tests/markerThumbnail.test.ts.

/** A thumbnail version: Unix ms (from the API), or the marker's updated_at date string. */
export type MarkerThumbnailVersionInput = number | string | null | undefined;

/**
 * Thumbnail version of a marker: Unix milliseconds of its updated_at (extra fraction digits,
 * as Go writes nanoseconds, are dropped like the server does), or null when unknown.
 */
export function markerThumbnailVersion(version: MarkerThumbnailVersionInput): number | null {
    if (typeof version === 'number') {
        return Number.isFinite(version) && version > 0 ? Math.trunc(version) : null;
    }
    if (typeof version !== 'string' || !version.trim()) return null;
    // Keep milliseconds only: Date.parse is only specified for 3 fraction digits
    const iso = version.trim().replace(/(\.\d{3})\d+/, '$1');
    const ms = Date.parse(iso);
    return Number.isFinite(ms) && ms > 0 ? ms : null;
}

/** URL of a marker thumbnail, versioned when the version is known. */
export function markerThumbnailUrl(
    markerId: number,
    version?: MarkerThumbnailVersionInput,
    animated = false,
): string {
    const base = `/marker-thumbnails/${markerId}${animated ? '/animated' : ''}`;
    const v = markerThumbnailVersion(version);
    return v == null ? base : `${base}?v=${v}`;
}

/**
 * Whether a marker's thumbnail version moved past `since` (a thumbnail was written after it).
 * A difference of 1 ms is ignored: the database rounds times to microseconds where Go
 * truncates them, and making a thumbnail always takes longer than that.
 */
export function markerThumbnailVersionChanged(
    since: MarkerThumbnailVersionInput,
    current: MarkerThumbnailVersionInput,
): boolean {
    const a = markerThumbnailVersion(since);
    const b = markerThumbnailVersion(current);
    if (b == null) return false;
    if (a == null) return true;
    return b - a > 1;
}
