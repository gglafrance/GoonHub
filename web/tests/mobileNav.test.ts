import { describe, expect, test } from 'bun:test';
import { buildMobileNav, mobileMenuPages } from '../app/utils/mobileNav';
import {
    resolveNavLayout,
    createNavFolder,
    hideNavItem,
    moveNavEntry,
} from '../app/utils/navLayout';
import { defaultNavLayout } from '../app/utils/navRegistry';
import type { PermissionName } from '../app/types/permissions';
import type { NavLayout } from '../app/types/navLayout';

const hasAll = () => true;
const hasOnly =
    (...granted: PermissionName[]) =>
    (p: PermissionName) =>
        granted.includes(p);

const build = (layout: NavLayout | null, has: (p: PermissionName) => boolean = hasAll) =>
    buildMobileNav(resolveNavLayout(layout, has));
const ids = (items: { id: string }[]) => items.map((i) => i.id);

describe('buildMobileNav', () => {
    test('default layout', () => {
        const nav = build(null);
        expect(ids(nav.quick)).toEqual(['command_palette', 'job_status', 'messages']);
        expect(ids(nav.tabs)).toEqual(['search', 'markers', 'playlists']);
        expect(nav.sections.map((s) => s.key)).toEqual(['pages', 'folder:manage']);
        expect(ids(nav.sections[0]!.items)).toEqual(['together', 'history', 'actors', 'studios']);
        expect(nav.sections[1]!.title).toBe('Manage');
        expect(ids(nav.sections[1]!.items)).toEqual(['triage', 'explorer', 'duplicates']);
        // messages is a quick icon, not a row; shortcuts are keyboard only
        expect(ids(nav.social)).toEqual(['friends', 'presence']);
        expect(ids(nav.toggles)).toEqual(['safe_mode', 'preview_sound']);
        expect(ids(nav.account)).toEqual(['settings', 'logout']);
        expect(nav.profile).toBe(true);
        const all = [
            nav.quick,
            nav.tabs,
            nav.social,
            nav.toggles,
            nav.account,
            ...nav.sections.map((s) => s.items),
        ]
            .flat()
            .map((i) => i.id);
        expect(all).not.toContain('shortcuts');
        expect(all).not.toContain('logo');
        expect(new Set(all).size).toBe(all.length);
    });

    test('permissions are applied', () => {
        const nav = build(null, hasOnly());
        expect(ids(nav.quick)).toEqual(['command_palette', 'messages']);
        expect(ids(nav.sections[0]!.items)).toEqual(['history', 'actors', 'studios']);
        expect(ids(nav.sections[1]!.items)).toEqual(['triage']);
    });

    test('the order of the layout picks the tabs', () => {
        let layout = defaultNavLayout();
        layout = moveNavEntry(layout, 'history', { kind: 'zone', bar: 'nav', zone: 'left' }, 0);
        const nav = build(layout);
        expect(ids(nav.tabs)).toEqual(['history', 'search', 'markers']);
        expect(ids(nav.sections[0]!.items)).toContain('playlists');
    });

    test('hidden items stay hidden', () => {
        let layout = defaultNavLayout();
        layout = hideNavItem(layout, 'command_palette');
        layout = hideNavItem(layout, 'messages');
        layout = hideNavItem(layout, 'search');
        layout = hideNavItem(layout, 'safe_mode');
        const nav = build(layout);
        expect(ids(nav.quick)).toEqual(['job_status']);
        expect(ids(nav.tabs)).toEqual(['markers', 'playlists', 'together']);
        expect(ids(nav.toggles)).toEqual(['preview_sound']);
        expect(ids(nav.social)).toEqual(['friends', 'presence']);
    });

    test('pages in folders come after the loose ones, folders keep their name', () => {
        let layout = defaultNavLayout();
        layout = createNavFolder(layout, 'Browse', 'heroicons:star', { bar: 'nav', zone: 'left' });
        const folderId = layout.folders.find((f) => f.name === 'Browse')!.id;
        for (const id of ['search', 'markers', 'playlists', 'together', 'history', 'actors']) {
            layout = moveNavEntry(layout, id, { kind: 'folder', folderId });
        }
        // the new folder is after Manage in the nav row: then its pages come after Manage's
        layout = moveNavEntry(
            layout,
            `folder:${folderId}`,
            {
                kind: 'zone',
                bar: 'nav',
                zone: 'left',
            },
            0,
        );
        const nav = build(layout);
        expect(ids(nav.tabs)).toEqual(['studios', 'search', 'markers']);
        const browse = nav.sections.find((s) => s.title === 'Browse')!;
        expect(browse.icon).toBe('heroicons:star');
        expect(ids(browse.items)).toEqual(['playlists', 'together', 'history', 'actors']);
        expect(nav.sections.find((s) => s.key === 'pages')).toBeUndefined();
    });

    test('menu pages', () => {
        const nav = build(null);
        const pages = mobileMenuPages(nav);
        expect(pages).toContain('/history');
        expect(pages).toContain('/duplicates');
        expect(pages).toContain('/friends');
        expect(pages).toContain('/settings');
        expect(pages).not.toContain('/search');
        // messages is in the top bar
        expect(pages).not.toContain('/messages');
    });
});
