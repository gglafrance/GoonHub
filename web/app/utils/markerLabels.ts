// Pure helpers of the server-wide marker labels (docs/MARKER_LABELS.md). Label names are
// unique ignoring case and surrounding spaces. Tests in web/tests/markerLabels.test.ts.

/** Longest label name (marker_labels.name is VARCHAR(100)). */
export const MARKER_LABEL_MAX = 100;

/** Matching key of a label name: trimmed, lower case. */
export function markerLabelKey(name: string): string {
    return name.trim().toLowerCase();
}

/** The label of the list named like typed (ignoring case and spaces), if any. */
export function findMarkerLabel<T extends { label: string }>(
    labels: readonly T[],
    typed: string,
): T | undefined {
    const key = markerLabelKey(typed);
    if (!key) return undefined;
    return labels.find((l) => markerLabelKey(l.label) === key);
}

/**
 * What a typed label resolves to when saving a marker: nothing (no label), an existing label
 * (its canonical spelling), a new label (only with marker_labels:manage: the API creates it)
 * or an unknown label (refused by the API without the permission).
 */
export type MarkerLabelPick =
    | { kind: 'empty' }
    | { kind: 'existing'; name: string }
    | { kind: 'create'; name: string }
    | { kind: 'unknown'; name: string };

export function pickMarkerLabel(
    typed: string,
    labels: readonly { label: string }[],
    canCreate: boolean,
): MarkerLabelPick {
    const name = typed.trim().slice(0, MARKER_LABEL_MAX);
    if (!name) return { kind: 'empty' };
    const existing = findMarkerLabel(labels, name);
    if (existing) return { kind: 'existing', name: existing.label };
    return canCreate ? { kind: 'create', name } : { kind: 'unknown', name };
}

/** Error shown when a user without marker_labels:manage types a label that does not exist. */
export function unknownMarkerLabelMessage(name: string): string {
    return `No label "${name}": pick an existing label (only label managers can create labels)`;
}

/**
 * The label to keep by default when merging: the one with the most markers, the oldest
 * (lowest id) on a tie. Undefined for no labels.
 */
export function defaultMarkerLabelMergeTarget<T extends { id: number; marker_count: number }>(
    labels: readonly T[],
): T | undefined {
    let best: T | undefined;
    for (const l of labels) {
        if (
            !best ||
            l.marker_count > best.marker_count ||
            (l.marker_count === best.marker_count && l.id < best.id)
        ) {
            best = l;
        }
    }
    return best;
}
