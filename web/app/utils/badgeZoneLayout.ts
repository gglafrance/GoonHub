import type { BadgeZone, BadgeZones } from '~/types/settings';

// Layout of a corner badge zone of the scene card. Pure functions, unit tested in
// web/tests/badgeZoneLayout.test.ts.
//
// A zone without `columns` (the templates saved before the grid) keeps its original flex layout:
// one column (vertical) or one row (horizontal). With `columns` (1 to 4) the items flow into a
// grid of that many columns, row by row (horizontal) or column by column (vertical): 2 columns
// and 6 items give 3 rows. The grid hugs its corner (right zones align their badges right,
// bottom zones align them down) and its columns shrink, badges truncated, when the zone would
// be wider than the thumbnail.

export type BadgeCorner = keyof BadgeZones;

// Same limits as the backend (data.MaxBadgeZoneItems, data.MaxBadgeZoneColumns)
export const MAX_BADGE_ZONE_ITEMS = 12;
export const MAX_BADGE_ZONE_COLUMNS = 4;

// Columns of a zone: 0 for the original layout (absent or out of range), else 1 to 4
export function zoneColumns(zone: Pick<BadgeZone, 'columns'>): number {
    const columns = zone.columns;
    if (typeof columns !== 'number' || !Number.isInteger(columns)) return 0;
    return columns >= 1 && columns <= MAX_BADGE_ZONE_COLUMNS ? columns : 0;
}

export function isGridZone(zone: Pick<BadgeZone, 'columns'>): boolean {
    return zoneColumns(zone) > 0;
}

// Rows of the grid for a number of items (0 for the original layout)
export function zoneRows(zone: Pick<BadgeZone, 'columns'>, itemCount: number): number {
    const columns = zoneColumns(zone);
    if (columns === 0 || itemCount <= 0) return 0;
    return Math.ceil(itemCount / columns);
}

export interface BadgeZoneLayout {
    grid: boolean;
    // Classes of the zone container
    class: string;
    // Grid template of the container (grid zones only)
    style?: Record<string, string>;
}

export function badgeZoneLayout(zone: BadgeZone, corner: BadgeCorner): BadgeZoneLayout {
    const columns = zoneColumns(zone);
    if (columns === 0) {
        return {
            grid: false,
            class:
                zone.direction === 'vertical'
                    ? 'flex gap-1 flex-col items-start'
                    : 'flex gap-1 flex-row items-end',
        };
    }
    // The cells space the badges with a margin (m-0.5, see BadgeItem `cell`) rather than a
    // grid gap, cancelled on the edges by -m-0.5: a track left empty (a badge with nothing to
    // show for that scene) then takes no room and the badges stay on their corner.
    const right = corner === 'top_right' || corner === 'bottom_right';
    const bottom = corner === 'bottom_left' || corner === 'bottom_right';
    const style: Record<string, string> = {
        gridTemplateColumns: `repeat(${columns}, minmax(0, max-content))`,
    };
    if (zone.direction === 'vertical') {
        style.gridAutoFlow = 'column';
        style.gridTemplateRows = `repeat(${Math.max(1, zoneRows(zone, zone.items.length))}, auto)`;
    }
    return {
        grid: true,
        class: [
            'grid -m-0.5',
            right ? 'justify-items-end' : 'justify-items-start',
            bottom ? 'items-end' : 'items-start',
        ].join(' '),
        style,
    };
}

// Corner on the same side (top or bottom) and on the same edge (left or right)
const BESIDE: Record<BadgeCorner, BadgeCorner> = {
    top_left: 'top_right',
    top_right: 'top_left',
    bottom_left: 'bottom_right',
    bottom_right: 'bottom_left',
};
const ABOVE_BELOW: Record<BadgeCorner, BadgeCorner> = {
    top_left: 'bottom_left',
    top_right: 'bottom_right',
    bottom_left: 'top_left',
    bottom_right: 'top_right',
};

// Size limits of the box of a grid zone on the thumbnail (undefined for the original layout,
// which keeps its unbounded size): the whole thumbnail less its insets, or half of it when the
// neighbouring zone on that axis shows badges too, so that the zones do not cover each other.
// The box clips what does not fit (the badge popovers are teleported, they are not clipped).
export function badgeZoneBoxStyle(
    badges: BadgeZones,
    corner: BadgeCorner,
): Record<string, string> | undefined {
    if (!isGridZone(badges[corner])) return undefined;
    const shared = (other: BadgeCorner) => badges[other].items.length > 0;
    return {
        maxWidth: shared(BESIDE[corner]) ? 'calc(50% - 0.5rem)' : 'calc(100% - 0.75rem)',
        maxHeight: shared(ABOVE_BELOW[corner]) ? 'calc(50% - 0.5rem)' : 'calc(100% - 0.75rem)',
        overflow: 'hidden',
    };
}

// Places of the items in the grid, for the editor: rows of item indexes (null for an empty
// cell). The original layout gives one column (vertical) or one row (horizontal).
export function zoneGridCells(zone: BadgeZone): (number | null)[][] {
    const count = zone.items.length;
    if (count === 0) return [];
    const columns = zoneColumns(zone) || (zone.direction === 'vertical' ? 1 : count);
    const rows = Math.ceil(count / columns);
    const cells: (number | null)[][] = Array.from({ length: rows }, () =>
        Array.from({ length: columns }, () => null),
    );
    for (let i = 0; i < count; i++) {
        const row = zone.direction === 'vertical' ? i % rows : Math.floor(i / columns);
        const col = zone.direction === 'vertical' ? Math.floor(i / rows) : i % columns;
        const line = cells[row];
        if (line) line[col] = i;
    }
    return cells;
}
