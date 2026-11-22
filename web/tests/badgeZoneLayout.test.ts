import { describe, expect, test } from 'bun:test';
import type { BadgeZone, BadgeZones } from '../app/types/settings';
import {
    MAX_BADGE_ZONE_COLUMNS,
    badgeZoneBoxStyle,
    badgeZoneLayout,
    isGridZone,
    zoneColumns,
    zoneGridCells,
    zoneRows,
} from '../app/utils/badgeZoneLayout';

const zone = (items: string[], direction: BadgeZone['direction'], columns?: number): BadgeZone =>
    columns === undefined ? { items, direction } : { items, direction, columns };

const six = ['rating', 'liked', 'views', 'resolution', 'duration', 'watched'];

describe('zoneColumns', () => {
    test('absent or out of range keeps the original layout', () => {
        expect(zoneColumns({})).toBe(0);
        expect(zoneColumns({ columns: 0 })).toBe(0);
        expect(zoneColumns({ columns: -1 })).toBe(0);
        expect(zoneColumns({ columns: MAX_BADGE_ZONE_COLUMNS + 1 })).toBe(0);
        expect(zoneColumns({ columns: 1.5 })).toBe(0);
        expect(isGridZone({})).toBe(false);
    });
    test('1 to 4 columns', () => {
        for (let n = 1; n <= MAX_BADGE_ZONE_COLUMNS; n++) {
            expect(zoneColumns({ columns: n })).toBe(n);
        }
        expect(isGridZone({ columns: 2 })).toBe(true);
    });
});

describe('zoneRows', () => {
    test('items flow into rows of the column count', () => {
        expect(zoneRows({ columns: 2 }, 6)).toBe(3);
        expect(zoneRows({ columns: 2 }, 5)).toBe(3);
        expect(zoneRows({ columns: 4 }, 12)).toBe(3);
        expect(zoneRows({ columns: 3 }, 0)).toBe(0);
        expect(zoneRows({}, 6)).toBe(0);
    });
});

describe('badgeZoneLayout', () => {
    test('old configs keep the exact flex classes', () => {
        expect(badgeZoneLayout(zone(['rating'], 'vertical'), 'top_left')).toEqual({
            grid: false,
            class: 'flex gap-1 flex-col items-start',
        });
        expect(badgeZoneLayout(zone(['duration'], 'horizontal'), 'bottom_right')).toEqual({
            grid: false,
            class: 'flex gap-1 flex-row items-end',
        });
    });
    test('horizontal grid fills rows', () => {
        const layout = badgeZoneLayout(zone(six, 'horizontal', 2), 'top_left');
        expect(layout.grid).toBe(true);
        expect(layout.style).toEqual({ gridTemplateColumns: 'repeat(2, minmax(0, max-content))' });
    });
    test('vertical grid fills columns', () => {
        const layout = badgeZoneLayout(zone(six, 'vertical', 2), 'top_left');
        expect(layout.style).toEqual({
            gridTemplateColumns: 'repeat(2, minmax(0, max-content))',
            gridAutoFlow: 'column',
            gridTemplateRows: 'repeat(3, auto)',
        });
    });
    test('the grid hugs its corner', () => {
        const classes = (corner: keyof BadgeZones) =>
            badgeZoneLayout(zone(six, 'horizontal', 3), corner).class.split(' ');
        expect(classes('top_left')).toEqual([
            'grid',
            '-m-0.5',
            'justify-items-start',
            'items-start',
        ]);
        expect(classes('top_right')).toEqual([
            'grid',
            '-m-0.5',
            'justify-items-end',
            'items-start',
        ]);
        expect(classes('bottom_left')).toEqual([
            'grid',
            '-m-0.5',
            'justify-items-start',
            'items-end',
        ]);
        expect(classes('bottom_right')).toEqual([
            'grid',
            '-m-0.5',
            'justify-items-end',
            'items-end',
        ]);
    });
});

describe('badgeZoneBoxStyle', () => {
    const badges = (over: Partial<BadgeZones>): BadgeZones => ({
        top_left: zone([], 'vertical'),
        top_right: zone([], 'vertical'),
        bottom_left: zone([], 'vertical'),
        bottom_right: zone([], 'horizontal'),
        ...over,
    });
    test('no limit for the original layout', () => {
        expect(badgeZoneBoxStyle(badges({ top_left: zone(six, 'vertical') }), 'top_left')).toBe(
            undefined,
        );
    });
    test('the whole thumbnail when alone', () => {
        expect(
            badgeZoneBoxStyle(badges({ top_left: zone(six, 'horizontal', 4) }), 'top_left'),
        ).toEqual({
            maxWidth: 'calc(100% - 0.75rem)',
            maxHeight: 'calc(100% - 0.75rem)',
            overflow: 'hidden',
        });
    });
    test('half of it beside or above another zone with badges', () => {
        const style = badgeZoneBoxStyle(
            badges({
                top_left: zone(six, 'horizontal', 4),
                top_right: zone(['watched'], 'vertical'),
                bottom_left: zone(['duration'], 'horizontal'),
            }),
            'top_left',
        );
        expect(style?.maxWidth).toBe('calc(50% - 0.5rem)');
        expect(style?.maxHeight).toBe('calc(50% - 0.5rem)');
    });
    test('an empty neighbour does not count', () => {
        const style = badgeZoneBoxStyle(
            badges({
                bottom_right: zone(six, 'vertical', 2),
                bottom_left: zone([], 'vertical', 2),
            }),
            'bottom_right',
        );
        expect(style?.maxWidth).toBe('calc(100% - 0.75rem)');
    });
});

describe('zoneGridCells', () => {
    test('original layout: one column or one row', () => {
        expect(zoneGridCells(zone(['a', 'b', 'c'], 'vertical'))).toEqual([[0], [1], [2]]);
        expect(zoneGridCells(zone(['a', 'b', 'c'], 'horizontal'))).toEqual([[0, 1, 2]]);
        expect(zoneGridCells(zone([], 'horizontal', 2))).toEqual([]);
    });
    test('horizontal fills rows, the last one may be incomplete', () => {
        expect(zoneGridCells(zone(['a', 'b', 'c', 'd', 'e'], 'horizontal', 2))).toEqual([
            [0, 1],
            [2, 3],
            [4, null],
        ]);
    });
    test('vertical fills columns, like grid-auto-flow: column', () => {
        expect(zoneGridCells(zone(six, 'vertical', 2))).toEqual([
            [0, 3],
            [1, 4],
            [2, 5],
        ]);
        expect(zoneGridCells(zone(['a', 'b', 'c', 'd', 'e'], 'vertical', 2))).toEqual([
            [0, 3],
            [1, 4],
            [2, null],
        ]);
    });
});
