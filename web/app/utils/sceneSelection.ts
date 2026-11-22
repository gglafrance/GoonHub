/**
 * The selection after a click on a scene's checkbox.
 *
 * A plain click toggles the scene. A shift-click (`range`) gives every scene between the
 * anchor (the last toggled scene) and the clicked one, both included and in list order, the
 * clicked scene's new state: selected if it was not, deselected if it was. Without a usable
 * anchor (none yet, the same scene, or a scene no longer in the list), a shift-click is a
 * plain toggle. The input set is left untouched.
 */
export function toggleSceneInSelection(
    selected: ReadonlySet<number>,
    orderedIds: readonly number[],
    sceneId: number,
    anchorId: number | null,
    range: boolean,
): Set<number> {
    const next = new Set(selected);
    const select = !next.has(sceneId);

    const from = range && anchorId !== null ? orderedIds.indexOf(anchorId) : -1;
    const to = from === -1 ? -1 : orderedIds.indexOf(sceneId);
    if (from === -1 || to === -1 || from === to) {
        if (select) next.add(sceneId);
        else next.delete(sceneId);
        return next;
    }

    const [start, end] = from < to ? [from, to] : [to, from];
    for (let i = start; i <= end; i++) {
        const id = orderedIds[i]!;
        if (select) next.add(id);
        else next.delete(id);
    }
    return next;
}
