import { describe, expect, test } from 'bun:test';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import {
    canPlaceNavEntry,
    cleanFolderName,
    completeNavLayout,
    createNavFolder,
    dissolveNavFolder,
    hideNavItem,
    isNavBarEmpty,
    moveNavEntry,
    resolveNavLayout,
    sanitizeNavLayout,
    shiftNavEntry,
    showNavItem,
    updateNavFolder,
    type ResolvedNavBar,
} from '../app/utils/navLayout';
import {
    MAX_NAV_FOLDERS,
    NAV_REGISTRY,
    defaultNavLayout,
    emptyNavLayout,
} from '../app/utils/navRegistry';
import type { PermissionName } from '../app/types/permissions';
import type { NavLayout } from '../app/types/navLayout';

const hasOnly =
    (...granted: PermissionName[]) =>
    (p: PermissionName) =>
        granted.includes(p);
const hasAll = () => true;

/** Entries of a resolved bar as ids, folders as "folder:<id>[items]". */
const ids = (bar: ResolvedNavBar) =>
    (['left', 'center', 'right'] as const).map((z) =>
        bar[z].map((e) =>
            e.kind === 'item' ? e.item.id : `${e.key}[${e.items.map((i) => i.id).join(',')}]`,
        ),
    );

// The header before the layouts existed (AppHeader + NavBar + ManageMenu)
const TODAY_TOP_RIGHT = [
    'command_palette',
    'profile',
    'shortcuts',
    'safe_mode',
    'preview_sound',
    'job_status',
    'presence',
    'messages',
    'friends',
    'settings',
    'logout',
];
const TODAY_NAV = ['search', 'markers', 'playlists', 'together', 'history', 'actors', 'studios'];

describe('default layout', () => {
    test("is today's header for a user with every permission", () => {
        const r = resolveNavLayout(null, hasAll);
        expect(ids(r.top)).toEqual([['logo'], [], TODAY_TOP_RIGHT]);
        expect(ids(r.nav)).toEqual([
            [...TODAY_NAV, 'folder:manage[triage,explorer,duplicates]'],
            [],
            [],
        ]);
    });

    test('keeps the permission gates of today', () => {
        const r = resolveNavLayout(null, hasOnly());
        expect(ids(r.top)[2]).toEqual(TODAY_TOP_RIGHT.filter((id) => id !== 'job_status'));
        expect(ids(r.nav)).toEqual([
            [...TODAY_NAV.filter((id) => id !== 'together'), 'folder:manage[triage]'],
            [],
            [],
        ]);
        const admin = resolveNavLayout(null, hasOnly('jobs:view_status', 'explorer:access'));
        expect(ids(admin.top)[2]).toContain('job_status');
        expect(ids(admin.nav)[0]).toContain('folder:manage[triage,explorer]');
    });

    test('the old Manage menu is the default folder, renamable and with its icon', () => {
        const layout = defaultNavLayout();
        expect(layout.folders).toEqual([
            {
                id: 'manage',
                name: 'Manage',
                icon: 'heroicons:wrench-screwdriver',
                items: ['triage', 'explorer', 'duplicates'],
            },
        ]);
        expect(layout.bars.nav.left.at(-1)).toBe('folder:manage');
        const renamed = updateNavFolder(layout, 'manage', 'Admin', 'heroicons:bolt');
        expect(renamed.folders[0]).toMatchObject({ name: 'Admin', icon: 'heroicons:bolt' });
    });

    test('a complete default layout is stable', () => {
        expect(completeNavLayout(null, hasAll)).toEqual(defaultNavLayout());
    });
});

describe('permissions', () => {
    test('an item the user lost disappears, without error', () => {
        const stored = defaultNavLayout();
        stored.bars.top.center = ['duplicates'];
        stored.folders[0]!.items = ['triage', 'explorer'];
        stored.hidden = ['job_status'];
        const r = resolveNavLayout(stored, hasOnly());
        expect(ids(r.top)[1]).toEqual([]);
        expect(ids(r.nav)[0]).toContain('folder:manage[triage]');
        expect(completeNavLayout(stored, hasOnly()).hidden).toEqual([]);
    });

    test('a folder without a visible item is not shown', () => {
        const stored = defaultNavLayout();
        stored.folders[0]!.items = ['explorer', 'duplicates'];
        stored.bars.nav.left = stored.bars.nav.left.filter((e) => e !== 'folder:manage');
        stored.bars.nav.right = ['folder:manage', 'triage'];
        expect(ids(resolveNavLayout(stored, hasOnly()).nav)[2]).toEqual(['triage']);
        expect(ids(resolveNavLayout(stored, hasAll).nav)[2]).toEqual([
            'folder:manage[explorer,duplicates]',
            'triage',
        ]);
    });

    test('an item granted later appears at its default place', () => {
        const stored = completeNavLayout(null, hasOnly());
        // saved without the gated items, then explorer:access is granted
        const r = completeNavLayout(stored, hasOnly('explorer:access', 'jobs:view_status'));
        expect(r.folders[0]!.items).toEqual(['triage', 'explorer']);
        expect(r.bars.top.right.at(-1)).toBe('job_status');
    });

    test('an item whose default folder was dissolved goes to its default zone', () => {
        const stored = dissolveNavFolder(completeNavLayout(null, hasOnly()), 'manage');
        const r = completeNavLayout(stored, hasOnly('duplicates:manage'));
        expect(r.folders).toEqual([]);
        expect(r.bars.nav.left.slice(-2)).toEqual(['triage', 'duplicates']);
    });
});

describe('sanitizeNavLayout', () => {
    test('drops what the server would refuse', () => {
        const raw = {
            version: 1,
            bars: {
                top: { left: ['logo', 'nope', 'search'], center: ['search'], right: [3] },
                nav: {
                    left: ['logo', 'folder:x', 'folder:x', 'folder:ghost'],
                    center: [],
                    right: [],
                },
            },
            folders: [
                { id: 'x', name: '  Mine ', icon: 'bad icon', items: ['job_status', 'history'] },
                { id: 'BAD', name: 'n', icon: '', items: [] },
                { id: 'lost', name: '', icon: '', items: ['actors'] },
            ],
            hidden: ['logout', 'history', 'markers'],
        };
        const out = sanitizeNavLayout(raw, hasAll);
        expect(out.bars.top).toEqual({ left: ['logo', 'search'], center: [], right: [] });
        expect(out.bars.nav.left).toEqual(['folder:x', 'folder:lost']);
        expect(out.folders).toEqual([
            { id: 'x', name: 'Mine', icon: '', items: ['history'] },
            { id: 'lost', name: 'Folder', icon: '', items: ['actors'] },
        ]);
        expect(out.hidden).toEqual(['markers']);
    });

    test('accepts garbage', () => {
        expect(sanitizeNavLayout(null, hasAll)).toEqual(emptyNavLayout());
        expect(sanitizeNavLayout('x', hasAll)).toEqual(emptyNavLayout());
        expect(completeNavLayout({} as NavLayout, hasAll).bars.top.left).toEqual(['logo']);
    });
});

describe('editor operations', () => {
    const base = () => completeNavLayout(null, hasAll);

    test('move between zones and bars', () => {
        let l = moveNavEntry(base(), 'history', { kind: 'zone', bar: 'top', zone: 'center' });
        expect(l.bars.top.center).toEqual(['history']);
        expect(l.bars.nav.left).not.toContain('history');
        l = moveNavEntry(l, 'messages', { kind: 'zone', bar: 'nav', zone: 'right' }, 0);
        expect(l.bars.nav.right).toEqual(['messages']);
        // the logo stays in the top bar
        expect(moveNavEntry(l, 'logo', { kind: 'zone', bar: 'nav', zone: 'left' })).toBe(l);
    });

    test('folders take folderable items only, never folders', () => {
        const l = base();
        const into = { kind: 'folder' as const, folderId: 'manage' };
        expect(moveNavEntry(l, 'search', into).folders[0]!.items.at(-1)).toBe('search');
        expect(canPlaceNavEntry(l, 'job_status', into)).toBe(false);
        expect(canPlaceNavEntry(l, 'logo', into)).toBe(false);
        const withFolder = createNavFolder(l, 'Other', '', { bar: 'top', zone: 'center' });
        expect(
            canPlaceNavEntry(withFolder, 'folder:manage', { kind: 'folder', folderId: 'f1' }),
        ).toBe(false);
        expect(canPlaceNavEntry(withFolder, 'folder:manage', { kind: 'hidden' })).toBe(false);
    });

    test('required items cannot be hidden; hidden ones come back at their default place', () => {
        const l = base();
        expect(hideNavItem(l, 'logout')).toBe(l);
        expect(hideNavItem(l, 'settings')).toBe(l);
        const hidden = hideNavItem(l, 'explorer');
        expect(hidden.hidden).toEqual(['explorer']);
        expect(hidden.folders[0]!.items).toEqual(['triage', 'duplicates']);
        const shown = showNavItem(hidden, 'explorer');
        expect(shown.hidden).toEqual([]);
        expect(shown.folders[0]!.items).toEqual(['triage', 'duplicates', 'explorer']);
    });

    test('shift within a list, bounded', () => {
        const l = base();
        expect(shiftNavEntry(l, 'markers', -1).bars.nav.left.slice(0, 2)).toEqual([
            'markers',
            'search',
        ]);
        expect(shiftNavEntry(l, 'search', -1)).toBe(l);
        expect(shiftNavEntry(l, 'folder:manage', 1)).toBe(l);
    });

    test('create, rename and dissolve folders', () => {
        let l = createNavFolder(base(), ' Library ', 'heroicons:film', {
            bar: 'top',
            zone: 'center',
        });
        expect(l.folders[1]).toEqual({
            id: 'f1',
            name: 'Library',
            icon: 'heroicons:film',
            items: [],
        });
        expect(l.bars.top.center).toEqual(['folder:f1']);
        l = moveNavEntry(l, 'actors', { kind: 'folder', folderId: 'f1' });
        l = moveNavEntry(l, 'studios', { kind: 'folder', folderId: 'f1' });
        expect(updateNavFolder(l, 'f1', '   ', '')).toBe(l);
        l = moveNavEntry(l, 'search', { kind: 'zone', bar: 'top', zone: 'center' });
        const dissolved = dissolveNavFolder(l, 'f1');
        expect(dissolved.bars.top.center).toEqual(['actors', 'studios', 'search']);
        expect(dissolved.folders.map((f) => f.id)).toEqual(['manage']);
    });

    test('folder limits', () => {
        let l = base();
        for (let i = 0; i < MAX_NAV_FOLDERS + 2; i++) {
            l = createNavFolder(l, `F${i}`, '', { bar: 'nav', zone: 'right' });
        }
        expect(l.folders).toHaveLength(MAX_NAV_FOLDERS);
        expect(cleanFolderName('a'.repeat(33))).toBeNull();
        expect(cleanFolderName('a\u0007')).toBeNull();
        expect(cleanFolderName('  ok ')).toBe('ok');
    });

    test('an empty nav row is detected', () => {
        const l = emptyNavLayout();
        l.bars.top.left = NAV_REGISTRY.filter((i) => i.bars.includes('top')).map((i) => i.id);
        expect(isNavBarEmpty(resolveNavLayout(l, hasAll).nav)).toBe(true);
    });
});

// The server has the same registry (internal/core/nav_layout_catalog.go): ids, permissions,
// bars and flags must match. Skipped when the Go sources are not next to web/.
const goCatalog = join(import.meta.dir, '../../internal/core/nav_layout_catalog.go');
const goRbac = join(import.meta.dir, '../../internal/core/rbac_catalog.go');

describe('registry in sync with the server', () => {
    test.skipIf(!existsSync(goCatalog))('same items, permissions, bars and flags', () => {
        const rbac = readFileSync(goRbac, 'utf8');
        const permValue = (name: string) =>
            new RegExp(`${name}\\s*=\\s*"([^"]+)"`).exec(rbac)?.[1] ?? `?${name}`;
        const go = readFileSync(goCatalog, 'utf8')
            .split('\n')
            .filter((line) => /^\s*\{ID: "/.test(line))
            .map((line) => ({
                id: /ID: "([^"]+)"/.exec(line)![1],
                permission: /Permission: (\w+)/.exec(line)?.[1],
                topOnly: line.includes('navBarsTopOnly'),
                required: line.includes('Required: true'),
                folderable: line.includes('Folderable: true'),
                defaultBar: /DefaultBar: NavBar(\w+)/.exec(line)![1]!.toLowerCase(),
                defaultZone: /DefaultZone: NavZone(\w+)/.exec(line)![1]!.toLowerCase(),
                defaultFolder: line.includes('DefaultFolder: DefaultNavFolderID')
                    ? 'manage'
                    : undefined,
            }));
        const web = NAV_REGISTRY.map((i) => ({
            id: i.id,
            permission: i.permission,
            topOnly: i.bars.length === 1 && i.bars[0] === 'top',
            required: !!i.required,
            folderable: !!i.folderable,
            defaultBar: i.defaultBar,
            defaultZone: i.defaultZone,
            defaultFolder: i.defaultFolder,
        }));
        expect(
            go.map((g) => ({ ...g, permission: g.permission && permValue(g.permission) })),
        ).toEqual(web);
    });
});
