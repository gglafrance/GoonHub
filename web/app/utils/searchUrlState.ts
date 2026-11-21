import type { LocationQuery } from 'vue-router';
import { SEARCH_STATUS_KEYS } from '~/stores/search';
import type { SearchStatus } from '~/stores/search';

export type SearchMatchType = 'broad' | 'strict' | 'frequency';

/** Everything that defines a search results page, as kept in the /search URL. */
export interface SearchUrlState {
    query: string;
    selectedTags: string[];
    selectedActors: string[];
    studio: string;
    minDuration: number;
    maxDuration: number;
    minSize: number;
    maxSize: number;
    status: SearchStatus;
    minDate: string;
    maxDate: string;
    resolution: string;
    sort: string;
    seed: number;
    page: number;
    liked: boolean;
    minRating: number;
    maxRating: number;
    minJizzCount: number;
    maxJizzCount: number;
    selectedMarkerLabels: string[];
    matchType: SearchMatchType;
}

const num = (raw: LocationQuery[string] | undefined): number => {
    const n = Number(queryString(raw));
    return Number.isFinite(n) && n > 0 ? n : 0;
};

/** /search URL query -> search state (absent params take the defaults). */
export function searchStateFromQuery(q: LocationQuery): SearchUrlState {
    const status = {} as SearchStatus;
    for (const k of SEARCH_STATUS_KEYS) status[k] = queryTriState(q[k]);
    return {
        query: queryString(q.q),
        selectedTags: queryList(q.tags),
        selectedActors: queryList(q.actors),
        studio: queryString(q.studio),
        minDuration: num(q.min_duration),
        maxDuration: num(q.max_duration),
        minSize: num(q.min_size),
        maxSize: num(q.max_size),
        status,
        minDate: queryString(q.min_date),
        maxDate: queryString(q.max_date),
        resolution: queryString(q.resolution),
        sort: queryString(q.sort),
        seed: querySeed(q.seed),
        page: queryPositiveInt(q.page, 1),
        liked: queryString(q.liked) === 'true',
        minRating: num(q.min_rating),
        maxRating: num(q.max_rating),
        minJizzCount: num(q.min_jizz_count),
        maxJizzCount: num(q.max_jizz_count),
        selectedMarkerLabels: queryList(q.marker_labels),
        matchType: queryEnum(q.match_type, ['broad', 'strict', 'frequency'] as const, 'broad'),
    };
}

/** Search state -> /search URL query (defaults left out). */
export function searchStateToQuery(s: SearchUrlState): Record<string, string> {
    const query: Record<string, string> = {};
    if (s.query) query.q = s.query;
    if (s.selectedTags.length > 0) query.tags = s.selectedTags.join(',');
    if (s.selectedActors.length > 0) query.actors = s.selectedActors.join(',');
    if (s.studio) query.studio = s.studio;
    if (s.minDuration > 0) query.min_duration = String(s.minDuration);
    if (s.maxDuration > 0) query.max_duration = String(s.maxDuration);
    if (s.minSize > 0) query.min_size = String(s.minSize);
    if (s.maxSize > 0) query.max_size = String(s.maxSize);
    for (const k of SEARCH_STATUS_KEYS) {
        const v = s.status[k];
        if (v !== null) query[k] = String(v);
    }
    if (s.minDate) query.min_date = s.minDate;
    if (s.maxDate) query.max_date = s.maxDate;
    if (s.resolution) query.resolution = s.resolution;
    if (s.sort) query.sort = s.sort;
    if (s.sort === 'random' && s.seed) query.seed = String(s.seed);
    if (s.page > 1) query.page = String(s.page);
    if (s.liked) query.liked = 'true';
    if (s.minRating > 0) query.min_rating = String(s.minRating);
    if (s.maxRating > 0) query.max_rating = String(s.maxRating);
    if (s.minJizzCount > 0) query.min_jizz_count = String(s.minJizzCount);
    if (s.maxJizzCount > 0) query.max_jizz_count = String(s.maxJizzCount);
    if (s.selectedMarkerLabels.length > 0) query.marker_labels = s.selectedMarkerLabels.join(',');
    if (s.matchType !== 'broad') query.match_type = s.matchType;
    return query;
}
