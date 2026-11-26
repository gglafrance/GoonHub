// Registry of the header items (both bars, docs/NAVIGATION.md). The server has the same ids,
// permissions, bars and flags (internal/core/nav_layout_catalog.go); web/tests/navLayout.test.ts
// reads the Go file to keep both in sync. The order is the default order (today's header).
import type { PermissionName } from '~/types/permissions';
import type { NavBarId, NavLayout, NavZoneId } from '~/types/navLayout';

export const NAV_LAYOUT_VERSION = 1;
/** SSE event telling the tabs of the user that their header layout changed (no data). */
export const NAV_LAYOUT_SSE_EVENT = 'settings:nav_layout';
export const NAV_FOLDER_REF_PREFIX = 'folder:';
export const DEFAULT_NAV_FOLDER_ID = 'manage';
export const MAX_NAV_FOLDERS = 12;
export const MAX_NAV_FOLDER_NAME_LENGTH = 32;
export const NAV_FOLDER_ID_PATTERN = /^[a-z0-9][a-z0-9_-]{0,31}$/;
export const NAV_FOLDER_ICON_PATTERN = /^heroicons:[a-z0-9]+(-[a-z0-9]+)*$/;

export const NAV_BARS: readonly NavBarId[] = ['top', 'nav'];
export const NAV_ZONES: readonly NavZoneId[] = ['left', 'center', 'right'];

export const NAV_BAR_LABELS: Record<NavBarId, string> = { top: 'Top bar', nav: 'Nav row' };
export const NAV_ZONE_LABELS: Record<NavZoneId, string> = {
    left: 'Left',
    center: 'Center',
    right: 'Right',
};

export interface NavItemDef {
    id: string;
    label: string;
    icon: string;
    /** Permission needed to see the item (none: every signed-in user) */
    permission?: PermissionName;
    /** Bars the item can go in */
    bars: readonly NavBarId[];
    /** Cannot be hidden (the logo, settings and logout) */
    required?: boolean;
    /** Can go in a folder (has a menu row rendering) */
    folderable?: boolean;
    defaultBar: NavBarId;
    defaultZone: NavZoneId;
    defaultFolder?: string;
    /** Page of a link item (rendered as a nav link); the menu rows of widgets use navItemRoute */
    to?: string;
}

const BOTH: readonly NavBarId[] = ['top', 'nav'];
const TOP_ONLY: readonly NavBarId[] = ['top'];

const topRight = { bars: BOTH, defaultBar: 'top', defaultZone: 'right' } as const;
const navLeft = { bars: BOTH, defaultBar: 'nav', defaultZone: 'left', folderable: true } as const;
const manage = { ...navLeft, defaultFolder: DEFAULT_NAV_FOLDER_ID } as const;

export const NAV_REGISTRY: readonly NavItemDef[] = [
    // Top bar
    {
        id: 'logo',
        label: 'GoonHub logo',
        icon: 'heroicons:home',
        bars: TOP_ONLY,
        required: true,
        defaultBar: 'top',
        defaultZone: 'left',
    },
    {
        id: 'command_palette',
        label: 'Search and commands',
        icon: 'heroicons:magnifying-glass',
        folderable: true,
        ...topRight,
    },
    {
        id: 'profile',
        label: 'My profile',
        icon: 'heroicons:user-circle',
        folderable: true,
        ...topRight,
    },
    {
        id: 'shortcuts',
        label: 'Keyboard shortcuts',
        icon: 'heroicons:command-line',
        folderable: true,
        ...topRight,
    },
    { id: 'safe_mode', label: 'Safe mode', icon: 'heroicons:eye', folderable: true, ...topRight },
    {
        id: 'preview_sound',
        label: 'Preview sound',
        icon: 'heroicons:speaker-wave',
        folderable: true,
        ...topRight,
    },
    {
        id: 'job_status',
        label: 'Job status',
        icon: 'heroicons:cpu-chip',
        permission: 'jobs:view_status',
        ...topRight,
    },
    { id: 'presence', label: 'Friends online', icon: 'heroicons:signal', ...topRight },
    {
        id: 'messages',
        label: 'Messages',
        icon: 'heroicons:chat-bubble-oval-left-ellipsis',
        folderable: true,
        to: '/messages',
        ...topRight,
    },
    {
        id: 'friends',
        label: 'Friends',
        icon: 'heroicons:users',
        folderable: true,
        to: '/friends',
        ...topRight,
    },
    {
        id: 'settings',
        label: 'Settings',
        icon: 'heroicons:cog-6-tooth',
        required: true,
        folderable: true,
        to: '/settings',
        ...topRight,
    },
    {
        id: 'logout',
        label: 'Logout',
        icon: 'heroicons:arrow-right-on-rectangle',
        required: true,
        folderable: true,
        ...topRight,
    },
    // Nav row: the user pages, then the Manage folder
    {
        id: 'search',
        label: 'Search',
        icon: 'heroicons:magnifying-glass',
        to: '/search',
        ...navLeft,
    },
    { id: 'markers', label: 'Markers', icon: 'heroicons:bookmark', to: '/markers', ...navLeft },
    {
        id: 'playlists',
        label: 'Playlists',
        icon: 'heroicons:queue-list',
        to: '/playlists',
        ...navLeft,
    },
    {
        id: 'together',
        label: 'Together',
        icon: 'heroicons:users',
        to: '/together',
        permission: 'goon2gether:use',
        ...navLeft,
    },
    { id: 'history', label: 'History', icon: 'heroicons:clock', to: '/history', ...navLeft },
    { id: 'actors', label: 'Actors', icon: 'heroicons:user-group', to: '/actors', ...navLeft },
    {
        id: 'studios',
        label: 'Studios',
        icon: 'heroicons:building-office-2',
        to: '/studios',
        ...navLeft,
    },
    { id: 'triage', label: 'Triage', icon: 'heroicons:bolt', to: '/triage', ...manage },
    {
        id: 'explorer',
        label: 'Explorer',
        icon: 'heroicons:folder',
        to: '/explorer',
        permission: 'explorer:access',
        ...manage,
    },
    {
        id: 'duplicates',
        label: 'Duplicates',
        icon: 'heroicons:document-duplicate',
        to: '/duplicates',
        permission: 'duplicates:manage',
        ...manage,
    },
];

/** Folders of the default layout: the former "Manage" menu. */
export const DEFAULT_NAV_FOLDERS = [
    { id: DEFAULT_NAV_FOLDER_ID, name: 'Manage', icon: 'heroicons:wrench-screwdriver' },
] as const;

/** Icons offered for the folders in the editor (the server accepts any heroicons name). */
export const NAV_FOLDER_ICONS: readonly string[] = [
    'heroicons:folder',
    'heroicons:wrench-screwdriver',
    'heroicons:squares-2x2',
    'heroicons:star',
    'heroicons:heart',
    'heroicons:fire',
    'heroicons:bolt',
    'heroicons:film',
    'heroicons:bookmark',
    'heroicons:users',
    'heroicons:sparkles',
    'heroicons:cog-6-tooth',
];

export const NAV_ITEMS_BY_ID: ReadonlyMap<string, NavItemDef> = new Map(
    NAV_REGISTRY.map((i) => [i.id, i]),
);

/** Whether a user may see the item. */
export function navItemAllowed(
    item: NavItemDef,
    has: (permission: PermissionName) => boolean,
): boolean {
    return !item.permission || has(item.permission);
}

/** The folder id of a zone entry, or null for an item entry. */
export function navFolderRefId(entry: string): string | null {
    return entry.startsWith(NAV_FOLDER_REF_PREFIX)
        ? entry.slice(NAV_FOLDER_REF_PREFIX.length)
        : null;
}

export function emptyNavLayout(): NavLayout {
    return {
        version: NAV_LAYOUT_VERSION,
        bars: {
            top: { left: [], center: [], right: [] },
            nav: { left: [], center: [], right: [] },
        },
        folders: [],
        hidden: [],
    };
}

/**
 * Layout of a user who never customized the header: today's header (the logo on the left of
 * the top bar, its icons on the right; the user pages in the nav row, then the Manage folder).
 */
export function defaultNavLayout(): NavLayout {
    const layout = emptyNavLayout();
    for (const f of DEFAULT_NAV_FOLDERS) layout.folders.push({ ...f, items: [] });
    for (const item of NAV_REGISTRY) {
        const folder = item.defaultFolder
            ? layout.folders.find((f) => f.id === item.defaultFolder)
            : undefined;
        const zone = layout.bars[item.defaultBar][item.defaultZone];
        if (folder) {
            const ref = NAV_FOLDER_REF_PREFIX + folder.id;
            if (!zone.includes(ref)) zone.push(ref);
            folder.items.push(item.id);
        } else {
            zone.push(item.id);
        }
    }
    return layout;
}
