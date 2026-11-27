// Figures of the custom lists (docs/PROFILES.md, "Figures"): which of the owner's figures the
// cards of a list show, and how. The selection mirrors core/profile_list_figures.go (the server
// decides what is sent; this is for the editor and the display). Pure helpers, tests in
// web/tests/listFigures.test.ts.
import type { ListFigureKind, ListFiguresSetting, ProfileSceneFigures } from '~/types/profile';
import type { SavedSearchFilters } from '~/types/saved_search';
import { ratingLabel, ratingStars } from './ratingHistogram';

/** The figure kinds, in the order of the secondary figures (the server's). */
export const LIST_FIGURE_KINDS: ListFigureKind[] = ['jizz', 'rating', 'liked', 'views'];

/** Figures shown under a card at most (the main one and one from the filters). */
export const LIST_FIGURE_MAX = 2;

export const LIST_FIGURE_LABELS: Record<ListFigureKind, string> = {
    jizz: 'O count',
    rating: 'Rating',
    liked: 'Like date',
    views: 'Views',
};

/** The main figure of a sort, null for a sort on no owner data. */
export function sortFigure(sort?: string): ListFigureKind | null {
    if (!sort) return null;
    if (sort.startsWith('jizz_count_') || sort.startsWith('last_jizz_')) return 'jizz';
    if (sort === 'my_rating_desc' || sort === 'my_rating_asc') return 'rating';
    if (sort === 'liked_at_desc') return 'liked';
    if (sort === 'my_views_desc' || sort === 'last_watched_desc') return 'views';
    return null;
}

/** The figures the filters select on (unrated or unwatched have nothing to show). */
function filterFigures(f: SavedSearchFilters): Record<ListFigureKind, boolean> {
    return {
        jizz: (f.min_jizz_count ?? 0) > 0 || (f.max_jizz_count ?? 0) > 0,
        rating: (f.min_rating ?? 0) > 0 || (f.max_rating ?? 0) > 0 || f.rated === true,
        liked: f.liked === true,
        views: f.watched === true,
    };
}

/** The figures of the "auto" setting: the sort's first, then the filters', at most two. */
export function autoListFigures(f: SavedSearchFilters): ListFigureKind[] {
    const out: ListFigureKind[] = [];
    const main = sortFigure(f.sort);
    if (main) out.push(main);
    const filters = filterFigures(f);
    for (const k of LIST_FIGURE_KINDS) {
        if (out.length >= LIST_FIGURE_MAX) break;
        if (filters[k] && out[0] !== k) out.push(k);
    }
    return out;
}

/** The figures a setting shows for criteria, main first. */
export function listFigures(
    setting: ListFiguresSetting | undefined,
    f: SavedSearchFilters,
): ListFigureKind[] {
    if (!setting || setting === 'auto') return autoListFigures(f);
    if (setting === 'none') return [];
    return LIST_FIGURE_KINDS.includes(setting) ? [setting] : [];
}

/** "Auto (O count, Like date)", "Auto (nothing for these criteria)". */
export function autoFiguresLabel(kinds: ListFigureKind[]): string {
    if (kinds.length === 0) return 'Auto (nothing for these criteria)';
    return `Auto (${kinds.map((k) => LIST_FIGURE_LABELS[k]).join(', ')})`;
}

/** The choices of the editor when the figures are shown. */
export function listFigureOptions(
    auto: ListFigureKind[],
): { value: Exclude<ListFiguresSetting, 'none'>; label: string }[] {
    return [
        { value: 'auto', label: autoFiguresLabel(auto) },
        ...LIST_FIGURE_KINDS.map((k) => ({ value: k, label: LIST_FIGURE_LABELS[k] })),
    ];
}

/** One figure under a card: stars, a value ("12 O", "34 views") and/or a dated part. */
export interface ListFigureLine {
    kind: ListFigureKind;
    /** Rating stars ("★★★½") */
    stars?: string;
    /** Bold value ("12 O", "34 views") */
    value?: string;
    /** Text before the date ("last", "Liked") */
    prefix?: string;
    /** ISO date shown relative */
    at?: string;
    /** Tooltip ("4.5 stars") */
    title?: string;
}

/** The lines shown under a card for the kinds of a list, skipping figures the scene lacks. */
export function listFigureLines(
    kinds: ListFigureKind[] | undefined,
    figures: ProfileSceneFigures | undefined,
): ListFigureLine[] {
    if (!kinds?.length || !figures) return [];
    const out: ListFigureLine[] = [];
    for (const kind of kinds.slice(0, LIST_FIGURE_MAX)) {
        switch (kind) {
            case 'jizz':
                if (figures.jizz_count)
                    out.push({
                        kind,
                        value: `${figures.jizz_count} O`,
                        prefix: figures.last_jizz_at ? 'last' : undefined,
                        at: figures.last_jizz_at,
                    });
                break;
            case 'rating':
                if (figures.rating)
                    out.push({
                        kind,
                        stars: ratingStars(figures.rating),
                        title: ratingLabel(figures.rating),
                    });
                break;
            case 'liked':
                if (figures.liked_at) out.push({ kind, prefix: 'Liked', at: figures.liked_at });
                break;
            case 'views':
                if (figures.views)
                    out.push({
                        kind,
                        value: `${figures.views} ${figures.views === 1 ? 'view' : 'views'}`,
                        prefix: figures.last_watched_at ? 'last' : undefined,
                        at: figures.last_watched_at,
                    });
                break;
        }
    }
    return out;
}
