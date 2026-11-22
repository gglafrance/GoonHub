// Total disk size of a scene selection, shown next to the selected count in the bulk toolbars
// ("12 selected · 18.4 GB"). Sizes are remembered per scene id as the cards are seen (list
// responses carry `size`); a selection that spans pages not loaded on the client (select all
// matching) is completed by the bulk toolbar's match-info request, which returns every
// selected scene's size. A size of 0 means unknown: it is counted apart, never summed as if
// the file were empty. Tests in web/tests/selectionSize.test.ts.

/** Size cache: scene id to size in bytes (0 = unknown). */
export type SceneSizeCache = Map<number, number>;

export interface SelectionSizeSummary {
    /** Sum of the known sizes, in bytes. */
    total: number;
    /** Selected scenes whose size is known (> 0). */
    known: number;
    /** Selected scenes whose size is 0 or invalid. */
    unknown: number;
    /** Selected scenes not in the cache yet (size not loaded on the client). */
    missing: number;
}

/** Records the sizes of the given scenes; returns true when the cache changed. */
export function rememberSceneSizes(
    cache: SceneSizeCache,
    scenes: readonly { id: number; size?: number | null }[],
): boolean {
    let changed = false;
    for (const scene of scenes) {
        const size = validSize(scene.size);
        if (cache.get(scene.id) !== size) {
            cache.set(scene.id, size);
            changed = true;
        }
    }
    return changed;
}

/** Sums the sizes of the selected scenes found in the cache. */
export function summarizeSelectionSize(
    ids: Iterable<number>,
    cache: ReadonlyMap<number, number>,
): SelectionSizeSummary {
    const summary: SelectionSizeSummary = { total: 0, known: 0, unknown: 0, missing: 0 };
    for (const id of ids) {
        const size = cache.get(id);
        if (size === undefined) summary.missing++;
        else if (size > 0) {
            summary.total += size;
            summary.known++;
        } else summary.unknown++;
    }
    return summary;
}

export interface SelectionSizeLabel {
    /** Formatted total ("18.4 GB"), or "size unknown" when no size is known. */
    size: string;
    /** Subtle note on the scenes left out of the total ("size of 3 unknown"), or null. */
    note: string | null;
}

/**
 * Label of a selection's total size, or null while some sizes are still being loaded
 * (`pending`), so a partial total is never shown as the real one. Once loading is over,
 * scenes still missing from the cache (deleted, request failed) count as unknown.
 */
export function selectionSizeLabel(
    summary: SelectionSizeSummary,
    formatSize: (bytes: number) => string,
    pending = false,
): SelectionSizeLabel | null {
    if (summary.missing > 0 && pending) return null;
    const unknown = summary.unknown + summary.missing;
    if (summary.known === 0) {
        return unknown > 0 ? { size: 'size unknown', note: null } : null;
    }
    return {
        size: formatSize(summary.total),
        note: unknown > 0 ? `size of ${unknown} unknown` : null,
    };
}

function validSize(size: number | null | undefined): number {
    return typeof size === 'number' && Number.isFinite(size) && size > 0 ? size : 0;
}
