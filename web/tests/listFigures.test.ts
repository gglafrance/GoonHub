import { describe, expect, test } from 'bun:test';
import {
    autoFiguresLabel,
    autoListFigures,
    listFigureLines,
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

describe('listFigureLines', () => {
    const at = '2026-11-20T10:00:00Z';
    test('formats each kind', () => {
        expect(
            listFigureLines(['jizz', 'rating'], { jizz_count: 12, last_jizz_at: at, rating: 3.5 }),
        ).toEqual([
            { kind: 'jizz', value: '12 O', prefix: 'last', at },
            { kind: 'rating', stars: '★★★½', title: '3.5 stars' },
        ]);
        expect(listFigureLines(['liked'], { liked_at: at })).toEqual([
            { kind: 'liked', prefix: 'Liked', at },
        ]);
        expect(listFigureLines(['views'], { views: 1 })).toEqual([
            { kind: 'views', value: '1 view', prefix: undefined, at: undefined },
        ]);
        expect(listFigureLines(['views'], { views: 34, last_watched_at: at })[0]?.value).toBe(
            '34 views',
        );
    });
    test('skips what the scene lacks or the server did not send', () => {
        expect(listFigureLines(['jizz', 'rating'], { rating: 4 })).toHaveLength(1);
        expect(listFigureLines(['jizz'], undefined)).toEqual([]);
        expect(listFigureLines([], { jizz_count: 3 })).toEqual([]);
        expect(listFigureLines(undefined, { jizz_count: 3 })).toEqual([]);
    });
    test('two lines at most', () => {
        const all = { jizz_count: 1, rating: 5, liked_at: at, views: 2 };
        expect(listFigureLines(['jizz', 'rating', 'liked', 'views'], all)).toHaveLength(2);
    });
});
