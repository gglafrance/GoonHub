// Pure helpers of the tag management panel (Settings > Tags): colors, search, and the merge
// of several tags into one. Tests in web/tests/tagManagement.test.ts.

/** Tag colors offered by the pickers (the first one is the default color of a new tag). */
export const TAG_COLOR_PRESETS = [
    '#6B7280',
    '#FF4D4D',
    '#F59E0B',
    '#8B5CF6',
    '#3B82F6',
    '#EC4899',
    '#F97316',
    '#14B8A6',
    '#22C55E',
    '#06B6D4',
    '#A78BFA',
    '#6366F1',
    '#EF4444',
] as const;

/**
 * Folds a tag name like the backend's duplicate detection: lower case, surrounding spaces
 * removed and inner runs of whitespace collapsed to one space.
 */
export function normalizeTagName(name: string): string {
    return name.trim().split(/\s+/).filter(Boolean).join(' ').toLowerCase();
}

/**
 * Parses a hex color typed by the user ("ff4d4d", "#FF4D4D", " #f4d4d4 "); returns it as
 * "#RRGGBB" in upper case, or null when it is not a 6-digit hex color.
 */
export function parseHexColor(input: string): string | null {
    const value = input.trim().replace(/^#/, '');
    if (!/^[0-9a-fA-F]{6}$/.test(value)) return null;
    return `#${value.toUpperCase()}`;
}

interface MergeCandidate {
    id: number;
    name: string;
    scene_count: number;
}

/** Tags whose normalized name contains the normalized query (all tags for an empty query). */
export function filterTagsByName<T extends { name: string }>(
    tags: readonly T[],
    query: string,
): T[] {
    const q = normalizeTagName(query);
    if (!q) return [...tags];
    return tags.filter((t) => normalizeTagName(t.name).includes(q));
}

/**
 * The tag to keep by default when merging: the one with the most scenes, the oldest (lowest
 * id) on a tie, as the backend suggests for duplicate groups. Undefined for no tags.
 */
export function defaultMergeTarget<T extends MergeCandidate>(tags: readonly T[]): T | undefined {
    let best: T | undefined;
    for (const t of tags) {
        if (
            !best ||
            t.scene_count > best.scene_count ||
            (t.scene_count === best.scene_count && t.id < best.id)
        ) {
            best = t;
        }
    }
    return best;
}

/** Source tag ids of a merge: the candidates other than the target, without duplicates. */
export function mergeSourceIds(candidateIds: readonly number[], targetId: number | null): number[] {
    const out: number[] = [];
    for (const id of candidateIds) {
        if (id !== targetId && !out.includes(id)) out.push(id);
    }
    return out;
}

/** "1 scene", "3 scenes": count with a pluralized noun. */
export function plural(count: number, noun: string): string {
    return `${count} ${noun}${count === 1 ? '' : 's'}`;
}
