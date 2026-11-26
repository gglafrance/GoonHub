// Layout logic of the header bars (docs/NAVIGATION.md): repairing a layout, placing the items
// it does not mention, resolving it for display, and the editor operations. Pure functions,
// tests in web/tests/navLayout.test.ts.
import type { PermissionName } from '~/types/permissions';
import type { NavBarId, NavFolder, NavLayout, NavZoneId } from '~/types/navLayout';
import {
    MAX_NAV_FOLDERS,
    MAX_NAV_FOLDER_NAME_LENGTH,
    NAV_BARS,
    NAV_FOLDER_ICON_PATTERN,
    NAV_FOLDER_ID_PATTERN,
    NAV_FOLDER_REF_PREFIX,
    NAV_ITEMS_BY_ID,
    NAV_REGISTRY,
    NAV_ZONES,
    defaultNavLayout,
    emptyNavLayout,
    navFolderRefId,
    navItemAllowed,
    type NavItemDef,
} from './navRegistry';

type Has = (permission: PermissionName) => boolean;

/** Where an entry is: a zone of a bar, a folder, or the hidden items. */
export type NavLocation =
    | { kind: 'zone'; bar: NavBarId; zone: NavZoneId }
    | { kind: 'folder'; folderId: string }
    | { kind: 'hidden' };

export function cloneNavLayout(layout: NavLayout): NavLayout {
    return {
        version: layout.version,
        bars: {
            top: {
                left: [...layout.bars.top.left],
                center: [...layout.bars.top.center],
                right: [...layout.bars.top.right],
            },
            nav: {
                left: [...layout.bars.nav.left],
                center: [...layout.bars.nav.center],
                right: [...layout.bars.nav.right],
            },
        },
        folders: layout.folders.map((f) => ({ ...f, items: [...f.items] })),
        hidden: [...layout.hidden],
    };
}

function asList(v: unknown): string[] {
    return Array.isArray(v) ? v.filter((e): e is string => typeof e === 'string') : [];
}

/** The folder that contains the entry's bar (a folder's items take the bar of the folder). */
function folderBar(layout: NavLayout, folderId: string): NavBarId | null {
    const ref = NAV_FOLDER_REF_PREFIX + folderId;
    for (const bar of NAV_BARS) {
        for (const zone of NAV_ZONES) {
            if (layout.bars[bar][zone].includes(ref)) return bar;
        }
    }
    return null;
}

/**
 * Repairs a layout the same way the server reads a stored one: unknown, duplicated, misplaced
 * and forbidden items are dropped, required items are never hidden, folders are checked
 * (unknown references dropped, unplaced folders put at the end of the nav row). Accepts any
 * value (a cached or server answer).
 */
export function sanitizeNavLayout(input: unknown, has: Has): NavLayout {
    const out = emptyNavLayout();
    const src = (input ?? {}) as Partial<NavLayout>;
    const seen = new Set<string>();

    const folders = new Map<string, NavFolder>();
    for (const raw of Array.isArray(src.folders) ? src.folders : []) {
        if (folders.size >= MAX_NAV_FOLDERS) break;
        if (!raw || typeof raw.id !== 'string' || !NAV_FOLDER_ID_PATTERN.test(raw.id)) continue;
        if (folders.has(raw.id)) continue;
        folders.set(raw.id, {
            id: raw.id,
            name: cleanFolderName(raw.name) ?? 'Folder',
            icon:
                typeof raw.icon === 'string' && NAV_FOLDER_ICON_PATTERN.test(raw.icon)
                    ? raw.icon
                    : '',
            items: asList(raw.items),
        });
    }

    const keepItem = (id: string, bar: NavBarId, inFolder: boolean): boolean => {
        const item = NAV_ITEMS_BY_ID.get(id);
        if (!item || seen.has(id) || !item.bars.includes(bar)) return false;
        if (inFolder && !item.folderable) return false;
        seen.add(id);
        return navItemAllowed(item, has);
    };

    const placedFolders = new Map<string, NavBarId>();
    for (const bar of NAV_BARS) {
        for (const zone of NAV_ZONES) {
            for (const entry of asList(src.bars?.[bar]?.[zone])) {
                const fid = navFolderRefId(entry);
                if (fid !== null) {
                    if (!folders.has(fid) || placedFolders.has(fid)) continue;
                    placedFolders.set(fid, bar);
                    out.bars[bar][zone].push(entry);
                } else if (keepItem(entry, bar, false)) {
                    out.bars[bar][zone].push(entry);
                }
            }
        }
    }

    for (const folder of folders.values()) {
        let bar = placedFolders.get(folder.id);
        if (!bar) {
            bar = 'nav';
            out.bars.nav.left.push(NAV_FOLDER_REF_PREFIX + folder.id);
        }
        const b = bar;
        out.folders.push({ ...folder, items: folder.items.filter((id) => keepItem(id, b, true)) });
    }

    for (const id of asList(src.hidden)) {
        const item = NAV_ITEMS_BY_ID.get(id);
        if (!item || seen.has(id) || item.required) continue;
        seen.add(id);
        if (navItemAllowed(item, has)) out.hidden.push(id);
    }
    return out;
}

/** The ids placed (in a zone or a folder) or hidden in a layout. */
function mentionedIds(layout: NavLayout): Set<string> {
    const ids = new Set<string>(layout.hidden);
    for (const bar of NAV_BARS) {
        for (const zone of NAV_ZONES) {
            for (const e of layout.bars[bar][zone]) if (navFolderRefId(e) === null) ids.add(e);
        }
    }
    for (const f of layout.folders) for (const id of f.items) ids.add(id);
    return ids;
}

/** Puts an item at its default place: its default folder when it still exists, else its zone. */
function placeAtDefault(layout: NavLayout, item: NavItemDef) {
    if (item.defaultFolder) {
        const folder = layout.folders.find((f) => f.id === item.defaultFolder);
        const bar = folder ? folderBar(layout, folder.id) : null;
        if (folder && bar && item.bars.includes(bar)) {
            folder.items.push(item.id);
            return;
        }
    }
    layout.bars[item.defaultBar][item.defaultZone].push(item.id);
}

/**
 * The full layout of a user: the stored one (or the default one), repaired, with every item
 * the user may see that it does not mention put at its default place (new items, items whose
 * permission was granted since). Every allowed item is then placed or hidden exactly once.
 */
export function completeNavLayout(layout: NavLayout | null | undefined, has: Has): NavLayout {
    const out = sanitizeNavLayout(layout ?? defaultNavLayout(), has);
    const mentioned = mentionedIds(out);
    for (const item of NAV_REGISTRY) {
        if (!mentioned.has(item.id) && navItemAllowed(item, has)) placeAtDefault(out, item);
    }
    return out;
}

export type ResolvedNavEntry =
    | { kind: 'item'; key: string; item: NavItemDef }
    | { kind: 'folder'; key: string; folder: NavFolder; items: NavItemDef[] };

export type ResolvedNavBar = Record<NavZoneId, ResolvedNavEntry[]>;

export interface ResolvedNavLayout {
    top: ResolvedNavBar;
    nav: ResolvedNavBar;
}

/**
 * What the header shows: the complete layout as item definitions, folders with their items;
 * a folder without a visible item is not shown.
 */
export function resolveNavLayout(
    layout: NavLayout | null | undefined,
    has: Has,
): ResolvedNavLayout {
    const full = completeNavLayout(layout, has);
    const folders = new Map(full.folders.map((f) => [f.id, f]));
    const resolveBar = (bar: NavBarId): ResolvedNavBar => {
        const out: ResolvedNavBar = { left: [], center: [], right: [] };
        for (const zone of NAV_ZONES) {
            for (const entry of full.bars[bar][zone]) {
                const fid = navFolderRefId(entry);
                if (fid !== null) {
                    const folder = folders.get(fid);
                    const items = (folder?.items ?? [])
                        .map((id) => NAV_ITEMS_BY_ID.get(id))
                        .filter((i): i is NavItemDef => !!i);
                    if (folder && items.length > 0) {
                        out[zone].push({ kind: 'folder', key: entry, folder, items });
                    }
                    continue;
                }
                const item = NAV_ITEMS_BY_ID.get(entry);
                if (item) out[zone].push({ kind: 'item', key: entry, item });
            }
        }
        return out;
    };
    return { top: resolveBar('top'), nav: resolveBar('nav') };
}

/** Whether a resolved bar has nothing to show. */
export function isNavBarEmpty(bar: ResolvedNavBar): boolean {
    return NAV_ZONES.every((z) => bar[z].length === 0);
}

// --- Editor operations: each returns a new layout, or the same one when not allowed ---

/** The container list of a location (null for an unknown folder). */
function listAt(layout: NavLayout, loc: NavLocation): string[] | null {
    if (loc.kind === 'zone') return layout.bars[loc.bar][loc.zone];
    if (loc.kind === 'hidden') return layout.hidden;
    return layout.folders.find((f) => f.id === loc.folderId)?.items ?? null;
}

/** Where an entry (item id or folder reference) is, and its index there. */
export function findNavEntry(
    layout: NavLayout,
    entry: string,
): { loc: NavLocation; index: number } | null {
    for (const bar of NAV_BARS) {
        for (const zone of NAV_ZONES) {
            const index = layout.bars[bar][zone].indexOf(entry);
            if (index >= 0) return { loc: { kind: 'zone', bar, zone }, index };
        }
    }
    for (const f of layout.folders) {
        const index = f.items.indexOf(entry);
        if (index >= 0) return { loc: { kind: 'folder', folderId: f.id }, index };
    }
    const index = layout.hidden.indexOf(entry);
    return index >= 0 ? { loc: { kind: 'hidden' }, index } : null;
}

function sameLocation(a: NavLocation, b: NavLocation): boolean {
    if (a.kind === 'zone' && b.kind === 'zone') return a.bar === b.bar && a.zone === b.zone;
    if (a.kind === 'folder' && b.kind === 'folder') return a.folderId === b.folderId;
    return a.kind === b.kind;
}

/** Whether an entry may go to a location (bar of the item, folders, required items). */
export function canPlaceNavEntry(layout: NavLayout, entry: string, loc: NavLocation): boolean {
    const fid = navFolderRefId(entry);
    if (fid !== null) {
        if (loc.kind !== 'zone') return false;
        const folder = layout.folders.find((f) => f.id === fid);
        // every item of the folder must be allowed in the target bar
        return (
            !!folder && folder.items.every((id) => NAV_ITEMS_BY_ID.get(id)?.bars.includes(loc.bar))
        );
    }
    const item = NAV_ITEMS_BY_ID.get(entry);
    if (!item) return false;
    if (loc.kind === 'hidden') return !item.required;
    if (loc.kind === 'zone') return item.bars.includes(loc.bar);
    const bar = folderBar(layout, loc.folderId);
    return !!item.folderable && !!bar && item.bars.includes(bar) && !!listAt(layout, loc);
}

/**
 * Moves an entry to a location, at an index (end when omitted). Moving within the same list
 * takes the index in the list without the entry.
 */
export function moveNavEntry(
    layout: NavLayout,
    entry: string,
    to: NavLocation,
    index?: number,
): NavLayout {
    const from = findNavEntry(layout, entry);
    if (!from || !canPlaceNavEntry(layout, entry, to)) return layout;
    const next = cloneNavLayout(layout);
    listAt(next, from.loc)!.splice(from.index, 1);
    const target = listAt(next, to)!;
    const at = index === undefined ? target.length : Math.max(0, Math.min(index, target.length));
    target.splice(at, 0, entry);
    if (sameLocation(from.loc, to) && from.index === at) return layout;
    return next;
}

/** Moves an entry one step left (-1) or right (+1) within its list. */
export function shiftNavEntry(layout: NavLayout, entry: string, delta: -1 | 1): NavLayout {
    const from = findNavEntry(layout, entry);
    if (!from) return layout;
    const len = listAt(layout, from.loc)!.length;
    const to = from.index + delta;
    if (to < 0 || to >= len) return layout;
    return moveNavEntry(layout, entry, from.loc, to);
}

/** Hides an item (not a required one). */
export function hideNavItem(layout: NavLayout, id: string): NavLayout {
    return moveNavEntry(layout, id, { kind: 'hidden' });
}

/** Shows a hidden item again at its default place. */
export function showNavItem(layout: NavLayout, id: string): NavLayout {
    const item = NAV_ITEMS_BY_ID.get(id);
    if (!item || !layout.hidden.includes(id)) return layout;
    const next = cloneNavLayout(layout);
    next.hidden = next.hidden.filter((h) => h !== id);
    placeAtDefault(next, item);
    return next;
}

// Control characters (Go's unicode.IsControl on the server: C0, DEL and C1)
// eslint-disable-next-line no-control-regex
const CONTROL_CHARS = /[\u0000-\u001f\u007f-\u009f]/;

/** A folder name as stored (trimmed), or null when it is not valid. */
export function cleanFolderName(name: unknown): string | null {
    if (typeof name !== 'string') return null;
    const trimmed = name.trim();
    if (!trimmed || [...trimmed].length > MAX_NAV_FOLDER_NAME_LENGTH) return null;
    return CONTROL_CHARS.test(trimmed) ? null : trimmed;
}

/** A free folder id (f1, f2...). */
export function nextNavFolderId(layout: NavLayout): string {
    const used = new Set(layout.folders.map((f) => f.id));
    let n = 1;
    while (used.has(`f${n}`)) n++;
    return `f${n}`;
}

/** Creates an empty folder at the end of a zone; same layout when the limit is reached. */
export function createNavFolder(
    layout: NavLayout,
    name: string,
    icon: string,
    at: { bar: NavBarId; zone: NavZoneId },
): NavLayout {
    const clean = cleanFolderName(name);
    if (!clean || layout.folders.length >= MAX_NAV_FOLDERS) return layout;
    const next = cloneNavLayout(layout);
    const id = nextNavFolderId(next);
    next.folders.push({
        id,
        name: clean,
        icon: NAV_FOLDER_ICON_PATTERN.test(icon) ? icon : '',
        items: [],
    });
    next.bars[at.bar][at.zone].push(NAV_FOLDER_REF_PREFIX + id);
    return next;
}

/** Renames a folder and sets its icon ('' = default folder icon). */
export function updateNavFolder(
    layout: NavLayout,
    folderId: string,
    name: string,
    icon: string,
): NavLayout {
    const clean = cleanFolderName(name);
    const index = layout.folders.findIndex((f) => f.id === folderId);
    if (!clean || index < 0) return layout;
    const next = cloneNavLayout(layout);
    next.folders[index] = {
        ...next.folders[index]!,
        name: clean,
        icon: icon === '' || NAV_FOLDER_ICON_PATTERN.test(icon) ? icon : next.folders[index]!.icon,
    };
    return next;
}

/** Removes a folder: its items take its place in its zone, in order. */
export function dissolveNavFolder(layout: NavLayout, folderId: string): NavLayout {
    const ref = NAV_FOLDER_REF_PREFIX + folderId;
    const pos = findNavEntry(layout, ref);
    const folder = layout.folders.find((f) => f.id === folderId);
    if (!folder || !pos || pos.loc.kind !== 'zone') return layout;
    const next = cloneNavLayout(layout);
    next.bars[pos.loc.bar][pos.loc.zone].splice(pos.index, 1, ...folder.items);
    next.folders = next.folders.filter((f) => f.id !== folderId);
    return next;
}

/** Whether two layouts are the same (for the editor's unsaved state). */
export function sameNavLayout(a: NavLayout, b: NavLayout): boolean {
    return JSON.stringify(a) === JSON.stringify(b);
}
