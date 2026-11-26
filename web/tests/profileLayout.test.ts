import { describe, expect, test } from 'bun:test';
import {
    activeTabId,
    countTabBlocks,
    criteriaDependencies,
    criteriaSummary,
    layoutIds,
    moveBlock,
    neighbourPosition,
    newLayoutId,
    newSection,
    profileSlug,
    sectionColumnClasses,
    setSectionTemplate,
    tabKeyOfPath,
    tabPath,
    templateColumnCount,
} from '../app/utils/profileLayout';
import type { ProfileSectionConfig, ProfileTabConfig, ProfileTabInfo } from '../app/types/profile';

const b = (id: string) => ({
    id,
    type: 'stats',
    visible: true,
    locked_placeholder: false,
    config: {},
});
const ids = (sections: ProfileSectionConfig[]) =>
    sections.map((s) => s.columns.map((c) => c.map((x) => x.id).join(',')).join('|'));

const sample = (): ProfileSectionConfig[] => [
    { id: 's1', template: '1-1', columns: [[b('a'), b('b')], [b('c')]] },
    { id: 's2', template: '1', columns: [[b('d')]] },
];

describe('templates', () => {
    test('columns and width classes', () => {
        expect(templateColumnCount('1-1-1')).toBe(3);
        expect(templateColumnCount('nope')).toBe(1);
        expect(sectionColumnClasses('2-1')).toEqual(['md:col-span-8', 'md:col-span-4']);
        expect(sectionColumnClasses('1')).toEqual(['md:col-span-12']);
    });

    test('a template with fewer columns moves their blocks to the last one', () => {
        const s = setSectionTemplate(
            { id: 's', template: '1-1-1', columns: [[b('a')], [b('b')], [b('c')]] },
            '1-1',
        );
        expect(ids([s])).toEqual(['a|b,c']);
        expect(ids([setSectionTemplate(s, '1-1-1')])).toEqual(['a|b,c|']);
        expect(newSection('x', '1-2').columns).toEqual([[], []]);
    });
});

describe('moving blocks', () => {
    test('within a column, across columns and sections', () => {
        const s = sample();
        expect(
            ids(
                moveBlock(
                    s,
                    { section: 0, column: 0, index: 0 },
                    { section: 0, column: 0, index: 2 },
                ),
            ),
        ).toEqual(['b,a|c', 'd']);
        expect(
            ids(
                moveBlock(
                    s,
                    { section: 0, column: 0, index: 1 },
                    { section: 0, column: 1, index: 0 },
                ),
            ),
        ).toEqual(['a|b,c', 'd']);
        expect(
            ids(
                moveBlock(
                    s,
                    { section: 1, column: 0, index: 0 },
                    { section: 0, column: 1, index: 9 },
                ),
            ),
        ).toEqual(['a,b|c,d', '']);
        // the input is left as it was
        expect(ids(s)).toEqual(['a,b|c', 'd']);
        // an unknown source changes nothing
        expect(
            ids(
                moveBlock(
                    s,
                    { section: 3, column: 0, index: 0 },
                    { section: 0, column: 0, index: 0 },
                ),
            ),
        ).toEqual(ids(s));
    });

    test('the arrows', () => {
        const s = sample();
        expect(neighbourPosition(s, { section: 0, column: 0, index: 0 }, 'up')).toBeNull();
        expect(neighbourPosition(s, { section: 0, column: 0, index: 1 }, 'up')).toEqual({
            section: 0,
            column: 0,
            index: 0,
        });
        expect(neighbourPosition(s, { section: 0, column: 0, index: 0 }, 'down')).toEqual({
            section: 0,
            column: 0,
            index: 2,
        });
        // the last block of a column goes down into the next section
        expect(neighbourPosition(s, { section: 0, column: 1, index: 0 }, 'down')).toEqual({
            section: 1,
            column: 0,
            index: 0,
        });
        expect(neighbourPosition(s, { section: 1, column: 0, index: 0 }, 'up')).toEqual({
            section: 0,
            column: 0,
            index: 2,
        });
        expect(neighbourPosition(s, { section: 0, column: 0, index: 0 }, 'right')).toEqual({
            section: 0,
            column: 1,
            index: 1,
        });
        expect(neighbourPosition(s, { section: 0, column: 1, index: 0 }, 'right')).toBeNull();
        expect(neighbourPosition(s, { section: 0, column: 0, index: 0 }, 'left')).toBeNull();
        const down = neighbourPosition(s, { section: 0, column: 0, index: 0 }, 'down')!;
        expect(ids(moveBlock(s, { section: 0, column: 0, index: 0 }, down))).toEqual([
            'b,a|c',
            'd',
        ]);
    });
});

describe('tabs', () => {
    const tabs: ProfileTabInfo[] = [
        { id: 'profile', kind: 'profile', title: 'Profile' },
        { id: 'likes', kind: 'likes', title: 'Likes' },
        { id: 't1', kind: 'custom', title: 'Top O', slug: 'top-o' },
    ];

    test('paths and the active tab of a route', () => {
        expect(tabPath('nade', tabs[0]!)).toBe('/u/nade');
        expect(tabPath('nade', tabs[1]!)).toBe('/u/nade/likes');
        expect(tabPath('nade', tabs[2]!)).toBe('/u/nade/t/top-o');
        expect(tabKeyOfPath('/u/nade')).toBe('');
        expect(tabKeyOfPath('/u/nade/ratings')).toBe('ratings');
        expect(tabKeyOfPath('/u/nade/t/top-o')).toBe('top-o');
        expect(tabKeyOfPath('/u/nade/lists/4')).toBe('-');
        expect(activeTabId('/u/nade', tabs)).toBe('profile');
        expect(activeTabId('/u/nade/t/top-o', tabs)).toBe('t1');
        expect(activeTabId('/u/nade/lists/4', tabs)).toBeUndefined();
    });

    test('ids, slugs and counts', () => {
        const config: ProfileTabConfig[] = [
            { id: 'profile', kind: 'profile', visible: true, sections: sample() },
        ];
        expect(layoutIds(config)).toEqual(['profile', 's1', 'a', 'b', 'c', 's2', 'd']);
        expect(countTabBlocks(config[0]!)).toBe(4);
        let n = 0;
        const seq = () => [0, 0, 0.5][n++] ?? 0.9;
        const first = newLayoutId('b', [], () => 0);
        expect(first).toBe('b00000');
        expect(newLayoutId('b', [first], seq)).not.toBe(first);
        expect(newLayoutId('b', [])).toMatch(/^b[a-z0-9]{5}$/);
        expect(profileSlug('Top O!')).toBe('top-o');
        expect(profileSlug('  Discover  ')).toBe('discover');
        expect(profileSlug('***')).toBe('');
    });
});

describe('custom list criteria', () => {
    test('dependencies, as the server computes them', () => {
        expect(criteriaDependencies({ selected_tags: ['a'], sort: 'view_count_desc' })).toEqual({
            items: [],
            size: false,
        });
        expect(criteriaDependencies({ sort: 'jizz_count_desc' }).items).toEqual(['jizz']);
        expect(criteriaDependencies({ rated: false }).items).toEqual(['ratings']);
        expect(criteriaDependencies({ liked: true, sort: 'liked_at_desc' }).items).toEqual([
            'liked',
        ]);
        expect(
            criteriaDependencies({
                watched: true,
                selected_marker_labels: ['kiss'],
                min_rating: 4,
                max_jizz_count: 3,
                liked: true,
            }).items,
        ).toEqual(['activity', 'recent_markers', 'liked', 'ratings', 'jizz']);
        expect(criteriaDependencies({ min_size: 100 }).size).toBe(true);
        expect(criteriaDependencies({ sort: 'size_desc' }).size).toBe(true);
    });

    test('summary', () => {
        expect(criteriaSummary({}, {})).toBe('Every video');
        expect(
            criteriaSummary({ selected_tags: ['a', 'b'], liked: true, sort: 'x' }, { x: 'Most O' }),
        ).toBe('tags: a, b · liked · sort: Most O');
    });
});
