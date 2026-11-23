// Layout and synchronisation of the duplicates compare player (every video of a group side by
// side). Each tile starts at its own matching segment; the shared position is an offset in
// seconds from the start of the segments, so seeking keeps every tile on the same moment.
// Tests in web/tests/duplicateCompare.test.ts.

/** Videos shown at once; larger groups are paged. */
export const COMPARE_PAGE_SIZE = 9;

interface CompareMember {
    duration: number;
    match_start?: number;
    match_end?: number;
}

/** Grid columns for n tiles: 1, 2 (2 to 4 tiles: 2 or 2x2) or 3 (5 to 9 tiles). */
export function compareGridColumns(n: number): number {
    if (n <= 1) return 1;
    if (n <= 4) return 2;
    return 3;
}

export function comparePageCount(n: number): number {
    return Math.max(1, Math.ceil(n / COMPARE_PAGE_SIZE));
}

/** Page (0-based) holding the item at index. */
export function comparePageOf(index: number): number {
    return index < 0 ? 0 : Math.floor(index / COMPARE_PAGE_SIZE);
}

export function comparePageSlice<T>(items: readonly T[], page: number): T[] {
    const start = Math.max(0, page) * COMPARE_PAGE_SIZE;
    return items.slice(start, start + COMPARE_PAGE_SIZE);
}

/** Where a tile starts: its matching segment, else the beginning. */
export function compareTileStart(member: CompareMember): number {
    return member.match_start ?? 0;
}

/** Time of a tile for a shared offset, kept inside the video. */
export function compareTileTime(member: CompareMember, offset: number): number {
    const t = compareTileStart(member) + offset;
    const end = member.duration > 0 ? member.duration : Number.POSITIVE_INFINITY;
    return Math.min(Math.max(0, t), end);
}

/** Shared offset of a tile playing at currentTime. */
export function compareOffsetOf(member: CompareMember, currentTime: number): number {
    return currentTime - compareTileStart(member);
}

/**
 * Range of the shared offset over the tiles: from the earliest start (every tile at its
 * beginning or before) to the latest end.
 */
export function compareOffsetRange(members: readonly CompareMember[]): {
    min: number;
    max: number;
} {
    if (members.length === 0) return { min: 0, max: 0 };
    let min = 0;
    let max = 0;
    for (const m of members) {
        const start = compareTileStart(m);
        min = Math.min(min, -start);
        max = Math.max(max, Math.max(0, m.duration) - start);
    }
    return { min, max };
}

/** Left and width (percent) of the matching segment on a tile's timeline, null without one. */
export function compareSegmentStyle(member: CompareMember): { left: string; width: string } | null {
    const { match_start: start, match_end: end, duration } = member;
    if (start === undefined || end === undefined || duration <= 0) return null;
    const left = Math.min(100, (start / duration) * 100);
    const width = Math.max(0.5, Math.min(100 - left, ((end - start) / duration) * 100));
    return { left: `${left}%`, width: `${width}%` };
}
