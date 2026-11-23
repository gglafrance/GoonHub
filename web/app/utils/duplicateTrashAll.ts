// What "Trash all" on the duplicates page would send to the trash: every video of the given
// groups not in the trash yet, the best one included. Sizes of 0 are unknown and counted apart.
// Groups not loaded on the client (selected on another page) are counted as missing.
// Tests in web/tests/duplicateTrashAll.test.ts.

interface TrashAllMember {
    scene_id: number;
    size: number;
    is_trashed: boolean;
}

export interface TrashAllSummary {
    /** Groups found among the given ids. */
    groups: number;
    /** Videos that would go to the trash (not trashed yet, each scene counted once). */
    videos: number;
    /** Total size of those videos whose size is known, in bytes. */
    size: number;
    /** Videos whose size is unknown (0 or invalid). */
    unknownSize: number;
    /** Videos already in the trash, left alone. */
    alreadyTrashed: number;
    /** Group ids not loaded on the client. */
    missingGroups: number;
}

export function summarizeTrashAll(
    groupIds: Iterable<number>,
    groupsById: ReadonlyMap<number, { members: readonly TrashAllMember[] }>,
): TrashAllSummary {
    const summary: TrashAllSummary = {
        groups: 0,
        videos: 0,
        size: 0,
        unknownSize: 0,
        alreadyTrashed: 0,
        missingGroups: 0,
    };
    const seen = new Set<number>();
    for (const id of groupIds) {
        const group = groupsById.get(id);
        if (!group) {
            summary.missingGroups++;
            continue;
        }
        summary.groups++;
        for (const member of group.members) {
            if (seen.has(member.scene_id)) continue;
            seen.add(member.scene_id);
            if (member.is_trashed) {
                summary.alreadyTrashed++;
                continue;
            }
            summary.videos++;
            if (Number.isFinite(member.size) && member.size > 0) summary.size += member.size;
            else summary.unknownSize++;
        }
    }
    return summary;
}

/** Result line of a bulk trash-all ("3 group(s) resolved, 12 video(s) moved to the trash"). */
export function describeBulkTrashAll(result: {
    resolved: number[];
    failed: unknown[];
    trashed_scenes: number;
    skipped_scenes: number;
}): string {
    return (
        `${result.resolved.length} group(s) resolved, ${result.trashed_scenes} video(s) moved to the trash` +
        (result.skipped_scenes
            ? `, ${result.skipped_scenes} already in the trash or missing`
            : '') +
        (result.failed.length ? `, ${result.failed.length} group(s) failed` : '')
    );
}
