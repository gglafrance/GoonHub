// Items of the top navigation (components/header/NavBar.vue): the user pages in the main row,
// the curation/management pages in the "Manage" menu at its end. Tests in web/tests/navItems.test.ts.
import type { PermissionName } from '~/types/permissions';

export type NavGroup = 'main' | 'manage';

export interface NavItem {
    to: string;
    label: string;
    icon: string;
    group: NavGroup;
    /** Permission needed to see the item (none: every signed-in user). */
    permission?: PermissionName;
}

/** Every nav item, in display order within its group. */
export const NAV_ITEMS: readonly NavItem[] = [
    { to: '/search', label: 'Search', icon: 'heroicons:magnifying-glass', group: 'main' },
    { to: '/markers', label: 'Markers', icon: 'heroicons:bookmark', group: 'main' },
    { to: '/playlists', label: 'Playlists', icon: 'heroicons:queue-list', group: 'main' },
    {
        to: '/together',
        label: 'Together',
        icon: 'heroicons:users',
        group: 'main',
        permission: 'goon2gether:use',
    },
    { to: '/history', label: 'History', icon: 'heroicons:clock', group: 'main' },
    { to: '/actors', label: 'Actors', icon: 'heroicons:user-group', group: 'main' },
    { to: '/studios', label: 'Studios', icon: 'heroicons:building-office-2', group: 'main' },
    { to: '/triage', label: 'Triage', icon: 'heroicons:bolt', group: 'manage' },
    {
        to: '/explorer',
        label: 'Explorer',
        icon: 'heroicons:folder',
        group: 'manage',
        permission: 'explorer:access',
    },
    {
        to: '/duplicates',
        label: 'Duplicates',
        icon: 'heroicons:document-duplicate',
        group: 'manage',
        permission: 'duplicates:manage',
    },
];

/** The items of a group the user may see, in order. */
export function visibleNavItems(
    items: readonly NavItem[],
    group: NavGroup,
    has: (permission: PermissionName) => boolean,
): NavItem[] {
    return items.filter((i) => i.group === group && (!i.permission || has(i.permission)));
}

/** Whether the current path is the item's page or one of its sub-pages. */
export function isNavItemActive(path: string, to: string): boolean {
    return path === to || path.startsWith(`${to}/`);
}

/** Whether the current path belongs to one of the items (the Manage button looks active). */
export function isAnyNavItemActive(path: string, items: readonly NavItem[]): boolean {
    return items.some((i) => isNavItemActive(path, i.to));
}

/**
 * Index of the menu item to focus after a key press (ArrowDown/ArrowUp wrap around, Home and
 * End jump to the ends); null for any other key. current -1 = no item focused.
 */
export function menuFocusIndex(current: number, length: number, key: string): number | null {
    if (length <= 0) return null;
    switch (key) {
        case 'ArrowDown':
            return current < 0 ? 0 : (current + 1) % length;
        case 'ArrowUp':
            return current < 0 ? length - 1 : (current - 1 + length) % length;
        case 'Home':
            return 0;
        case 'End':
            return length - 1;
        default:
            return null;
    }
}

/**
 * Horizontal position of a menu panel opening under its button: aligned on the button's left
 * edge and extending to the right, shifted left when it would overflow the viewport.
 */
export function menuPanelLeft(
    buttonLeft: number,
    panelWidth: number,
    viewportWidth: number,
    padding = 8,
): number {
    const maxLeft = viewportWidth - panelWidth - padding;
    return Math.max(padding, Math.min(buttonLeft, maxLeft));
}
