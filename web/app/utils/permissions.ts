// Pure helpers of the roles and permissions UI (contract: docs/RBAC.md). Unit tested in
// web/tests/permissions.test.ts.
import type {
    CatalogPermission,
    EffectivePermission,
    PermissionChoice,
    PermissionSetting,
} from '~/types/permissions';

/** Effective permissions of the signed-in user, as returned by GET /auth/permissions. */
export interface PermissionGrants {
    superuser: boolean;
    permissions: readonly string[];
}

/** A superuser role has every permission; otherwise the permission must be listed. */
export function grantsPermission(state: PermissionGrants, name: string): boolean {
    return state.superuser || state.permissions.includes(name);
}

export interface PermissionGroup {
    group: string;
    permissions: CatalogPermission[];
}

/** Groups the catalog by `group`, keeping the catalog order of groups and of permissions. */
export function groupPermissions(catalog: readonly CatalogPermission[]): PermissionGroup[] {
    const groups: PermissionGroup[] = [];
    const byName = new Map<string, PermissionGroup>();
    for (const permission of catalog) {
        const name = permission.group || 'Other';
        let group = byName.get(name);
        if (!group) {
            group = { group: name, permissions: [] };
            byName.set(name, group);
            groups.push(group);
        }
        group.permissions.push(permission);
    }
    return groups;
}

/** Tri-state of a permission in a settings list: no setting means "inherit". */
export function choiceOf(
    settings: readonly PermissionSetting[],
    permissionId: number,
): PermissionChoice {
    return settings.find((s) => s.permission_id === permissionId)?.effect ?? 'inherit';
}

/**
 * Returns the settings with the permission set to the choice ("inherit" removes the setting).
 * The result is sorted by permission id so that two equal lists compare equal.
 */
export function withChoice(
    settings: readonly PermissionSetting[],
    permissionId: number,
    choice: PermissionChoice,
): PermissionSetting[] {
    const rest = settings.filter((s) => s.permission_id !== permissionId);
    if (choice !== 'inherit') rest.push({ permission_id: permissionId, effect: choice });
    return sortSettings(rest);
}

export function sortSettings(settings: readonly PermissionSetting[]): PermissionSetting[] {
    return [...settings].sort((a, b) => a.permission_id - b.permission_id);
}

/** True when both lists hold the same settings, whatever their order. */
export function sameSettings(
    a: readonly PermissionSetting[],
    b: readonly PermissionSetting[],
): boolean {
    if (a.length !== b.length) return false;
    const sortedA = sortSettings(a);
    const sortedB = sortSettings(b);
    return sortedA.every(
        (s, i) => s.permission_id === sortedB[i]!.permission_id && s.effect === sortedB[i]!.effect,
    );
}

/** Effective values indexed by permission name. */
export function effectiveByName(
    effective: readonly EffectivePermission[],
): Map<string, EffectivePermission> {
    return new Map(effective.map((e) => [e.name, e]));
}

/** Human label of an effective value's source. */
export function describeSource(source: string): string {
    if (source === 'superuser') return 'superuser role';
    if (source === 'role') return 'role setting';
    if (source === 'override') return 'user override';
    if (source.startsWith('parent:')) return `from ${source.slice('parent:'.length)}`;
    return 'not set';
}

/**
 * Ids of the roles that inherit from the given role, directly or not. They cannot become its
 * parents (that would be a cycle); the API refuses cycles anyway.
 */
export function descendantRoleIds(
    roles: readonly { id: number; parent_ids: readonly number[] }[],
    roleId: number,
): Set<number> {
    const found = new Set<number>();
    const queue = [roleId];
    while (queue.length > 0) {
        const current = queue.shift()!;
        for (const role of roles) {
            if (role.parent_ids.includes(current) && !found.has(role.id) && role.id !== roleId) {
                found.add(role.id);
                queue.push(role.id);
            }
        }
    }
    return found;
}

/** Badge classes of a role: the superuser role stands out, the others share one style. */
export function roleBadgeClass(role: { name: string; is_superuser?: boolean } | string): string {
    const superuser = typeof role === 'string' ? role === 'admin' : !!role.is_superuser;
    const name = typeof role === 'string' ? role : role.name;
    if (superuser || name === 'admin') return 'bg-lava/15 text-lava border-lava/30';
    return 'bg-white/5 text-white/70 border-white/10';
}

/** Role names accepted by the API (POST/PUT /admin/roles). */
export const ROLE_NAME_PATTERN = /^[a-z0-9][a-z0-9_-]{1,49}$/;

export function isValidRoleName(name: string): boolean {
    return ROLE_NAME_PATTERN.test(name);
}

/**
 * Roles that can be chosen as parents of a role (`roleId` null when creating one): every
 * other role except those inheriting from it, which would make a cycle.
 */
export function parentCandidates<T extends { id: number; parent_ids: readonly number[] }>(
    roles: readonly T[],
    roleId: number | null,
): T[] {
    if (roleId === null) return [...roles];
    const descendants = descendantRoleIds(roles, roleId);
    return roles.filter((r) => r.id !== roleId && !descendants.has(r.id));
}

/** Default role offered when creating a user: `user` when it exists, else a regular role. */
export function defaultRoleName(roles: readonly { name: string; is_superuser: boolean }[]): string {
    if (roles.some((r) => r.name === 'user')) return 'user';
    return (roles.find((r) => !r.is_superuser) ?? roles[0])?.name ?? '';
}
