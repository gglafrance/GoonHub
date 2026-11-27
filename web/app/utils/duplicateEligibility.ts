// Eligible members of the duplicate groups (duplicates page): every member is eligible
// (checked) by default; the client keeps, per group, the scenes unchecked as "not a duplicate".
// Before a resolve or a trash-all, the unchecked scenes are removed from their group
// (POST /admin/duplicates/:id/exclude) so those actions only touch the eligible ones.
// Tests in web/tests/duplicateEligibility.test.ts.

export type UncheckedByGroup = ReadonlyMap<number, ReadonlySet<number>>;

interface EligibilityGroup<M extends { scene_id: number } = { scene_id: number }> {
    id: number;
    members: readonly M[];
}

/** Checks or unchecks one member of a group. */
export function toggleUnchecked(
    state: UncheckedByGroup,
    groupId: number,
    sceneId: number,
): Map<number, Set<number>> {
    const next = cloneState(state);
    const set = new Set(next.get(groupId) ?? []);
    if (set.has(sceneId)) set.delete(sceneId);
    else set.add(sceneId);
    if (set.size) next.set(groupId, set);
    else next.delete(groupId);
    return next;
}

/** Replaces the unchecked members of a group (empty: every member eligible again). */
export function setGroupUnchecked(
    state: UncheckedByGroup,
    groupId: number,
    sceneIds: Iterable<number>,
): Map<number, Set<number>> {
    const next = cloneState(state);
    const set = new Set(sceneIds);
    if (set.size) next.set(groupId, set);
    else next.delete(groupId);
    return next;
}

/** Scene IDs of the unchecked members of a group, in member order (current members only). */
export function uncheckedMembers(state: UncheckedByGroup, group: EligibilityGroup): number[] {
    const set = state.get(group.id);
    if (!set?.size) return [];
    return group.members.filter((m) => set.has(m.scene_id)).map((m) => m.scene_id);
}

/** Members of a group still checked as eligible. */
export function eligibleMembers<M extends { scene_id: number }>(
    state: UncheckedByGroup,
    group: EligibilityGroup<M>,
): M[] {
    const set = state.get(group.id);
    if (!set?.size) return [...group.members];
    return group.members.filter((m) => !set.has(m.scene_id));
}

/**
 * Drops, for the given (reloaded) groups, the unchecked scenes that are no longer members, and
 * the groups that are no longer unresolved. Other groups are kept as they are.
 */
export function pruneUnchecked(
    state: UncheckedByGroup,
    groups: Iterable<EligibilityGroup & { status?: string }>,
): Map<number, Set<number>> {
    const next = cloneState(state);
    for (const group of groups) {
        const set = next.get(group.id);
        if (!set) continue;
        if (group.status !== undefined && group.status !== 'unresolved') {
            next.delete(group.id);
            continue;
        }
        const members = new Set(group.members.map((m) => m.scene_id));
        const kept = new Set([...set].filter((id) => members.has(id)));
        if (kept.size) next.set(group.id, kept);
        else next.delete(group.id);
    }
    return next;
}

export interface GroupExclusion {
    groupId: number;
    /** Unchecked scenes to remove from the group. */
    sceneIds: number[];
    /** Members left in the group (fewer than 2: the server dismisses the group). */
    remaining: number;
}

export interface ExclusionPlan {
    /** Groups with unchecked members, to exclude before acting on them. */
    exclusions: GroupExclusion[];
    /** Groups whose members are all unchecked: left untouched (nothing to act on). */
    nothingEligible: number[];
    /** Unchecked videos, summed over the groups. */
    uncheckedVideos: number;
}

/** What must be excluded before acting on the given groups (only groups loaded on the client). */
export function planExclusions(
    groupIds: Iterable<number>,
    groupsById: ReadonlyMap<number, EligibilityGroup>,
    state: UncheckedByGroup,
): ExclusionPlan {
    const plan: ExclusionPlan = { exclusions: [], nothingEligible: [], uncheckedVideos: 0 };
    for (const id of groupIds) {
        const group = groupsById.get(id);
        if (!group) continue;
        const sceneIds = uncheckedMembers(state, group);
        if (sceneIds.length === 0) continue;
        if (sceneIds.length >= group.members.length) {
            plan.nothingEligible.push(id);
            continue;
        }
        plan.exclusions.push({
            groupId: id,
            sceneIds,
            remaining: group.members.length - sceneIds.length,
        });
        plan.uncheckedVideos += sceneIds.length;
    }
    return plan;
}

/**
 * Runs the exclusions of a plan, one group at a time (stops at the first failure). Returns the
 * groups the server dismissed because fewer than 2 members were left.
 */
export async function runExclusionPlan(
    plan: ExclusionPlan,
    exclude: (groupId: number, sceneIds: number[]) => Promise<{ dismissed: boolean }>,
): Promise<Set<number>> {
    const dismissed = new Set<number>();
    for (const e of plan.exclusions) {
        const result = await exclude(e.groupId, e.sceneIds);
        if (result.dismissed) dismissed.add(e.groupId);
    }
    return dismissed;
}

function cloneState(state: UncheckedByGroup): Map<number, Set<number>> {
    const next = new Map<number, Set<number>>();
    for (const [id, set] of state) next.set(id, new Set(set));
    return next;
}
