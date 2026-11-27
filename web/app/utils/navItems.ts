// Helpers of the header links and dropdown folders (components/header/NavLink.vue, NavFolder.vue).
// The items themselves are in utils/navRegistry.ts, their placement in utils/navLayout.ts.
// Tests in web/tests/navItems.test.ts.

/** Whether the current path is the item's page or one of its sub-pages. */
export function isNavItemActive(path: string, to: string): boolean {
    return path === to || path.startsWith(`${to}/`);
}

/** Whether the current path belongs to one of the items (the folder button looks active). */
export function isAnyNavItemActive(path: string, items: readonly { to?: string }[]): boolean {
    return items.some((i) => !!i.to && isNavItemActive(path, i.to));
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
