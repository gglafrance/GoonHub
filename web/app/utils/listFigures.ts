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

/** One figure of a list card: an icon and a short value, the details in the tooltip. */
export interface ListFigureBadge {
    kind: ListFigureKind;
    /** Nuxt Icon name */
    icon: string;
    /** Short value: "12", "4.5", "2d" (the icon tells what it is, no unit) */
    text: string;
    /** Tooltip: "12 O, last 3 days ago" */
    title: string;
}

export const LIST_FIGURE_ICONS: Record<ListFigureKind, string> = {
    jizz: 'fluent-emoji-high-contrast:sweat-droplets',
    rating: 'heroicons:star-solid',
    liked: 'heroicons:heart-solid',
    views: 'heroicons:eye',
};

const UNITS: [number, string][] = [
    [365 * 86400, 'y'],
    [30 * 86400, 'mo'],
    [7 * 86400, 'w'],
    [86400, 'd'],
    [3600, 'h'],
    [60, 'm'],
];

/** Compact age of a date: "now", "5m", "3h", "2d", "4w", "6mo", "1y". */
export function shortAgo(iso: string, now: number = Date.now()): string {
    const seconds = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000));
    for (const [size, unit] of UNITS) {
        if (seconds >= size) return `${Math.floor(seconds / size)}${unit}`;
    }
    return 'now';
}

/** Long age for a tooltip: "3 days ago". */
function longAgo(iso: string, now: number): string {
    const s = shortAgo(iso, now);
    if (s === 'now') return 'just now';
    const n = parseInt(s, 10);
    const unit = s.replace(String(n), '');
    const names: Record<string, string> = {
        y: 'year',
        mo: 'month',
        w: 'week',
        d: 'day',
        h: 'hour',
        m: 'minute',
    };
    const name = names[unit] ?? unit;
    return `${n} ${name}${n === 1 ? '' : 's'} ago`;
}

/** The badges of a list card for the kinds of the list, skipping figures the scene lacks. */
export function listFigureBadges(
    kinds: ListFigureKind[] | undefined,
    figures: ProfileSceneFigures | undefined,
    now: number = Date.now(),
): ListFigureBadge[] {
    if (!kinds?.length || !figures) return [];
    const out: ListFigureBadge[] = [];
    for (const kind of kinds.slice(0, LIST_FIGURE_MAX)) {
        const icon = LIST_FIGURE_ICONS[kind];
        switch (kind) {
            case 'jizz':
                if (figures.jizz_count) {
                    const last = figures.last_jizz_at
                        ? `, last ${longAgo(figures.last_jizz_at, now)}`
                        : '';
                    out.push({
                        kind,
                        icon,
                        text: String(figures.jizz_count),
                        title: `${figures.jizz_count} O${last}`,
                    });
                }
                break;
            case 'rating':
                if (figures.rating)
                    out.push({
                        kind,
                        icon,
                        text: String(figures.rating),
                        title: `${ratingLabel(figures.rating)} ${ratingStars(figures.rating)}`,
                    });
                break;
            case 'liked':
                if (figures.liked_at)
                    out.push({
                        kind,
                        icon,
                        text: shortAgo(figures.liked_at, now),
                        title: `Liked ${longAgo(figures.liked_at, now)}`,
                    });
                break;
            case 'views':
                if (figures.views) {
                    const last = figures.last_watched_at
                        ? `, last ${longAgo(figures.last_watched_at, now)}`
                        : '';
                    out.push({
                        kind,
                        icon,
                        text: String(figures.views),
                        title: `${figures.views} ${figures.views === 1 ? 'view' : 'views'}${last}`,
                    });
                }
                break;
        }
    }
    return out;
}
