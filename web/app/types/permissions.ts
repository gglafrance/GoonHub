// Permission catalog and RBAC API shapes (contract: docs/RBAC.md).

/** Every permission of the catalog checked by the UI. */
export type PermissionName =
    | 'scenes:view'
    | 'scenes:upload'
    | 'scenes:edit'
    | 'scenes:trash'
    | 'scenes:delete'
    | 'scenes:reprocess'
    | 'scenes:share'
    | 'markers:create'
    | 'marker_labels:manage'
    | 'tags:manage'
    | 'actors:manage'
    | 'actors:merge'
    | 'studios:manage'
    | 'studios:merge'
    | 'playlists:create'
    | 'playlists:edit'
    | 'playlists:delete'
    | 'playlists:view_public'
    | 'files:view_size'
    | 'files:view_paths'
    | 'files:view_technical'
    | 'explorer:access'
    | 'jobs:view_status'
    | 'jobs:manage'
    | 'users:manage'
    | 'roles:manage'
    | 'settings:manage'
    | 'storage:manage'
    | 'duplicates:manage'
    | 'trash:manage'
    | 'library:health'
    | 'library:insights'
    | 'porndb:use'
    | 'import:use'
    | 'stats:view_streams'
    | 'social:use'
    | 'goon2gether:use'
    | 'goon2gether:host'
    | 'goon2gether:moderate'
    | 'emotes:manage';

/** GET /api/v1/auth/permissions: the signed-in user's effective permissions. */
export interface MyPermissionsResponse {
    role: string;
    superuser: boolean;
    permissions: string[];
}

/** A role or user setting for one permission. No setting means "inherit". */
export type PermissionEffect = 'allow' | 'deny';

/** Tri-state shown in the permission editors. */
export type PermissionChoice = 'inherit' | PermissionEffect;

export interface PermissionSetting {
    permission_id: number;
    effect: PermissionEffect;
}

/** Catalog entry (GET /api/v1/admin/permissions). */
export interface CatalogPermission {
    id: number;
    name: string;
    group: string;
    description: string;
}

/**
 * Effective value of a permission for a role or a user. `source` is `superuser`, `role` (own
 * setting), `override` (user only), `parent:<role name>` or `none`.
 */
export interface EffectivePermission {
    name: string;
    granted: boolean;
    source: string;
}

export interface RoleSummary {
    id: number;
    name: string;
    description: string;
    is_system: boolean;
    is_superuser: boolean;
    parent_ids: number[];
    settings: PermissionSetting[];
    user_count: number;
}

export interface RoleInput {
    name: string;
    description: string;
    parent_ids: number[];
}

/** GET /api/v1/admin/users/:id/permissions */
export interface UserPermissionsResponse {
    role: string;
    overrides: PermissionSetting[];
    effective: EffectivePermission[];
}
