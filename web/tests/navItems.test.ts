import { describe, expect, test } from 'bun:test';
import {
    NAV_ITEMS,
    isAnyNavItemActive,
    isNavItemActive,
    menuFocusIndex,
    menuPanelLeft,
    visibleNavItems,
} from '../app/utils/navItems';
import type { PermissionName } from '../app/types/permissions';

const hasOnly =
    (...granted: PermissionName[]) =>
    (p: PermissionName) =>
        granted.includes(p);

const labels = (items: { label: string }[]) => items.map((i) => i.label);

describe('visibleNavItems', () => {
    test('main row order, gated items hidden without their permission', () => {
        expect(labels(visibleNavItems(NAV_ITEMS, 'main', hasOnly()))).toEqual([
            'Search',
            'Markers',
            'Playlists',
            'History',
            'Actors',
            'Studios',
        ]);
        expect(labels(visibleNavItems(NAV_ITEMS, 'main', hasOnly('goon2gether:use')))).toEqual([
            'Search',
            'Markers',
            'Playlists',
            'Together',
            'History',
            'Actors',
            'Studios',
        ]);
    });

    test('manage menu keeps the permission gates', () => {
        expect(labels(visibleNavItems(NAV_ITEMS, 'manage', hasOnly()))).toEqual(['Triage']);
        expect(
            labels(
                visibleNavItems(
                    NAV_ITEMS,
                    'manage',
                    hasOnly('explorer:access', 'duplicates:manage'),
                ),
            ),
        ).toEqual(['Triage', 'Explorer', 'Duplicates']);
    });

    test('an empty group when nothing is visible', () => {
        const gated = NAV_ITEMS.filter((i) => i.permission);
        expect(visibleNavItems(gated, 'manage', hasOnly())).toEqual([]);
    });
});

describe('isNavItemActive', () => {
    test('the page and its sub-pages, not a mere prefix', () => {
        expect(isNavItemActive('/explorer', '/explorer')).toBe(true);
        expect(isNavItemActive('/explorer/3/folder', '/explorer')).toBe(true);
        expect(isNavItemActive('/explorers', '/explorer')).toBe(false);
        expect(isNavItemActive('/', '/explorer')).toBe(false);
    });

    test('the manage button is active on one of its pages', () => {
        const manage = visibleNavItems(NAV_ITEMS, 'manage', () => true);
        expect(isAnyNavItemActive('/duplicates', manage)).toBe(true);
        expect(isAnyNavItemActive('/explorer/1', manage)).toBe(true);
        expect(isAnyNavItemActive('/search', manage)).toBe(false);
    });
});

describe('menuFocusIndex', () => {
    test('arrows wrap, Home and End jump, other keys do nothing', () => {
        expect(menuFocusIndex(-1, 3, 'ArrowDown')).toBe(0);
        expect(menuFocusIndex(0, 3, 'ArrowDown')).toBe(1);
        expect(menuFocusIndex(2, 3, 'ArrowDown')).toBe(0);
        expect(menuFocusIndex(-1, 3, 'ArrowUp')).toBe(2);
        expect(menuFocusIndex(0, 3, 'ArrowUp')).toBe(2);
        expect(menuFocusIndex(1, 3, 'Home')).toBe(0);
        expect(menuFocusIndex(0, 3, 'End')).toBe(2);
        expect(menuFocusIndex(0, 3, 'Enter')).toBeNull();
        expect(menuFocusIndex(-1, 0, 'ArrowDown')).toBeNull();
    });
});

describe('menuPanelLeft', () => {
    test('aligned on the button, kept inside the viewport', () => {
        expect(menuPanelLeft(500, 200, 1200)).toBe(500);
        expect(menuPanelLeft(1100, 200, 1200)).toBe(992);
        expect(menuPanelLeft(2, 200, 1200)).toBe(8);
        // narrower viewport than the panel: stuck to the left padding
        expect(menuPanelLeft(100, 400, 300)).toBe(8);
    });
});
