/**
 * URL state of the triage page. The queue filters use the same query keys as the search page
 * (has_tags=false, tags=a,b, sort=...), plus `i` (1-based position) and `scene` (current
 * scene ID). `saved=<uuid>` loads a saved search on entry.
 */
import type { LocationQuery, LocationQueryValue } from 'vue-router';
import type { SavedSearchFilters } from '~/types/saved_search';
import { SEARCH_STATUS_KEYS } from '~/stores/search';
import type { TriageParams } from '~/composables/useTriageQueue';

type QueryValue = LocationQueryValue | LocationQueryValue[] | undefined;

const POSITION_KEYS = ['i', 'scene', 'saved'];

const str = (v: QueryValue): string => (Array.isArray(v) ? v[0] || '' : v || '');
const num = (v: QueryValue): number | undefined => {
    const n = Number(str(v));
    return Number.isFinite(n) && n > 0 ? n : undefined;
};
const list = (v: QueryValue): string[] | undefined => {
    const s = str(v);
    return s ? s.split(',') : undefined;
};

/** True when the URL carries queue filters (anything but the position keys). */
export const hasTriageFilters = (query: LocationQuery): boolean =>
    Object.keys(query).some((k) => !POSITION_KEYS.includes(k));

/** Search filters described by the URL (inverse of the search store's getSearchParams). */
export const triageFiltersFromQuery = (query: LocationQuery): SavedSearchFilters => {
    const filters: SavedSearchFilters = {
        query: str(query.q) || undefined,
        match_type: str(query.match_type) || undefined,
        selected_tags: list(query.tags),
        selected_actors: list(query.actors),
        studio: str(query.studio) || undefined,
        resolution: str(query.resolution) || undefined,
        min_duration: num(query.min_duration),
        max_duration: num(query.max_duration),
        min_size: num(query.min_size),
        max_size: num(query.max_size),
        min_date: str(query.min_date) || undefined,
        max_date: str(query.max_date) || undefined,
        liked: str(query.liked) === 'true' || undefined,
        min_rating: num(query.min_rating),
        max_rating: num(query.max_rating),
        min_jizz_count: num(query.min_jizz_count),
        max_jizz_count: num(query.max_jizz_count),
        selected_marker_labels: list(query.marker_labels),
        sort: str(query.sort) || undefined,
    };
    for (const k of SEARCH_STATUS_KEYS) {
        const v = str(query[k]);
        if (v === 'true') filters[k] = true;
        else if (v === 'false') filters[k] = false;
    }
    return filters;
};

export const triageSeedFromQuery = (query: LocationQuery): number | undefined => num(query.seed);

/** URL query for the queue params and the current position. */
export const triageQueryFromParams = (
    params: TriageParams,
    position: number,
    sceneId: number | null,
): Record<string, string> => {
    const out: Record<string, string> = {};
    for (const [k, v] of Object.entries(params)) {
        if (v !== undefined && v !== '' && v !== 0) out[k] = String(v);
    }
    if (position > 1) out.i = String(position);
    if (sceneId !== null) out.scene = String(sceneId);
    return out;
};
