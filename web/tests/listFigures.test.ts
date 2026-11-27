import { describe, expect, test } from 'bun:test';
import {
    autoFiguresLabel,
    autoListFigures,
    listFigureBadges,
    LIST_FIGURE_ICONS,
    shortAgo,
    listFigureOptions,
    listFigures,
    sortFigure,
} from '../app/utils/listFigures';
import { CUSTOM_LIST_PRESETS } from '../app/utils/profileLayout';

describe('sortFigure', () => {
    test('maps the owner sorts to their figure', () => {
        expect(sortFigure('jizz_count_desc')).toBe('jizz');
        expect(sortFigure('jizz_count_asc')).toBe('jizz');
        expect(sortFigure('last_jizz_desc')).toBe('jizz');
        expect(sortFigure('my_rating_asc')).toBe('rating');
        expect(sortFigure('liked_at_desc')).toBe('liked');
        expect(sortFigure('my_views_desc')).toBe('views');
        expect(sortFigure('last_watched_desc')).toBe('views');
    });
    test('library sorts have no figure', () => {
        expect(sortFigure('created_at_desc')).toBeNull();
        expect(sortFigure('view_count_desc')).toBeNull();
        expect(sortFigure(undefined)).toBeNull();
    });
});

describe('autoListFigures', () => {
    test('the sort first, then the filters, two at most', () => {
        expect(autoListFigures({ sort: 'jizz_count_desc', liked: true })).toEqual([
            'jizz',
            'liked',
        ]);
        expect(
            autoListFigures({
                sort: 'my_views_desc',
                liked: true,
                min_rating: 3,
                min_jizz_count: 1,
            }),
        ).toEqual(['views', 'jizz']);
        expect(autoListFigures({ liked: true, min_rating: 3 })).toEqual(['rating', 'liked']);
    });
    test('a figure is not repeated', () => {
        expect(autoListFigures({ sort: 'jizz_count_desc', min_jizz_count: 1 })).toEqual(['jizz']);
    });
    test('absence filters and library criteria show nothing', () => {
        expect(autoListFigures({ rated: false, watched: false })).toEqual([]);
        expect(autoListFigures({ sort: 'created_at_desc', selected_tags: ['x'] })).toEqual([]);
    });
    test('the presets: on for owner data, off for Newest', () => {
        const byTitle = Object.fromEntries(
            CUSTOM_LIST_PRESETS.map((p) => [p.title, autoListFigures(p.filters)]),
        );
        expect(byTitle['Top O videos']).toEqual(['jizz']);
        expect(byTitle['Top rated']).toEqual(['rating']);
        expect(byTitle['Recently liked']).toEqual(['liked']);
        expect(byTitle['Most watched']).toEqual(['views']);
        expect(byTitle['Newest in the library']).toEqual([]);
    });
});

describe('listFigures', () => {
    const topO = { sort: 'jizz_count_desc' };
    test('auto, none and an explicit kind', () => {
        expect(listFigures('auto', topO)).toEqual(['jizz']);
        expect(listFigures(undefined, topO)).toEqual(['jizz']);
        expect(listFigures('none', topO)).toEqual([]);
        expect(listFigures('rating', topO)).toEqual(['rating']);
    });
    test('editor options name what auto shows', () => {
        expect(autoFiguresLabel(['jizz', 'liked'])).toBe('Auto (O count, Like date)');
        expect(autoFiguresLabel([])).toBe('Auto (nothing for these criteria)');
        expect(listFigureOptions(['jizz']).map((o) => o.value)).toEqual([
            'auto',
            'jizz',
            'rating',
            'liked',
            'views',
        ]);
    });
});

describe('listFigureBadges', () => {
    const now = Date.parse('2026-11-23T10:00:00Z');
    const at = '2026-11-20T10:00:00Z';
    test('icon and short value, details in the tooltip (no unit next to the icon)', () => {
        expect(
            listFigureBadges(
                ['jizz', 'rating'],
                { jizz_count: 12, last_jizz_at: at, rating: 3.5 },
                now,
            ),
        ).toEqual([
            {
                kind: 'jizz',
                icon: LIST_FIGURE_ICONS.jizz,
                text: '12',
                title: '12 O, last 3 days ago',
            },
            {
                kind: 'rating',
                icon: LIST_FIGURE_ICONS.rating,
                text: '3.5',
                title: '3.5 stars ★★★½',
            },
        ]);
        expect(listFigureBadges(['liked'], { liked_at: at }, now)[0]).toMatchObject({
            text: '3d',
            title: 'Liked 3 days ago',
        });
        expect(listFigureBadges(['views'], { views: 1 }, now)[0]).toMatchObject({
            text: '1',
            title: '1 view',
        });
    });
    test('skips what the scene lacks or the server did not send', () => {
        expect(listFigureBadges(['jizz', 'rating'], { rating: 4 })).toHaveLength(1);
        expect(listFigureBadges(['jizz'], undefined)).toEqual([]);
        expect(listFigureBadges([], { jizz_count: 3 })).toEqual([]);
        expect(listFigureBadges(undefined, { jizz_count: 3 })).toEqual([]);
    });
    test('two badges at most', () => {
        const all = { jizz_count: 1, rating: 5, liked_at: at, views: 2 };
        expect(listFigureBadges(['jizz', 'rating', 'liked', 'views'], all)).toHaveLength(2);
    });
});

describe('shortAgo', () => {
    const now = Date.parse('2026-11-23T10:00:00Z');
    test('compact units', () => {
        expect(shortAgo('2026-11-23T09:59:50Z', now)).toBe('now');
        expect(shortAgo('2026-11-23T09:55:00Z', now)).toBe('5m');
        expect(shortAgo('2026-11-23T07:00:00Z', now)).toBe('3h');
        expect(shortAgo('2026-11-21T10:00:00Z', now)).toBe('2d');
        expect(shortAgo('2026-10-26T10:00:00Z', now)).toBe('4w');
        expect(shortAgo('2026-05-23T10:00:00Z', now)).toBe('6mo');
        expect(shortAgo('2025-11-01T10:00:00Z', now)).toBe('1y');
    });
});
