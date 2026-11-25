import type { TranscodeCacheClearResponse, TranscodeCacheStats } from '~/types/admin';
import type {
    CatalogPermission,
    EffectivePermission,
    PermissionSetting,
    RoleInput,
    RoleSummary,
    UserPermissionsResponse,
} from '~/types/permissions';
import type {
    TrashBulkResult,
    TrashFacets,
    TrashIdsResponse,
    TrashListResponse,
} from '~/types/trash';

/**
 * Admin API operations: users, roles, permissions management, trash, transcode cache.
 */
export const useApiAdmin = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    const fetchAdminUsers = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/users?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createUser = async (username: string, password: string, role: string) => {
        const response = await fetch('/api/v1/admin/users', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ username, password, role }),
        });
        return handleResponse(response);
    };

    const updateUserRole = async (userId: number, role: string) => {
        const response = await fetch(`/api/v1/admin/users/${userId}/role`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ role }),
        });
        return handleResponse(response);
    };

    const updateUserShareMarkers = async (userId: number, shareMarkers: boolean) => {
        const response = await fetch(`/api/v1/admin/users/${userId}/share-markers`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ share_markers: shareMarkers }),
        });
        return handleResponse(response);
    };

    const resetUserPassword = async (userId: number, newPassword: string) => {
        const response = await fetch(`/api/v1/admin/users/${userId}/password`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ new_password: newPassword }),
        });
        return handleResponse(response);
    };

    const deleteUser = async (userId: number) => {
        const response = await fetch(`/api/v1/admin/users/${userId}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchRoles = async (): Promise<{ roles: RoleSummary[] }> => {
        const response = await fetch('/api/v1/admin/roles', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Permission catalog (deprecated permissions excluded), in catalog order
    const fetchPermissions = async (): Promise<{ permissions: CatalogPermission[] }> => {
        const response = await fetch('/api/v1/admin/permissions', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createRole = async (input: RoleInput): Promise<RoleSummary> => {
        const response = await fetch('/api/v1/admin/roles', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(input),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateRole = async (roleId: number, input: RoleInput): Promise<RoleSummary> => {
        const response = await fetch(`/api/v1/admin/roles/${roleId}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(input),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const deleteRole = async (roleId: number) => {
        const response = await fetch(`/api/v1/admin/roles/${roleId}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Replaces the role's own settings (absent = inherit)
    const setRolePermissions = async (roleId: number, settings: PermissionSetting[]) => {
        const response = await fetch(`/api/v1/admin/roles/${roleId}/permissions`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ settings }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchRoleEffective = async (
        roleId: number,
    ): Promise<{ permissions: EffectivePermission[] }> => {
        const response = await fetch(`/api/v1/admin/roles/${roleId}/effective`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchUserPermissions = async (userId: number): Promise<UserPermissionsResponse> => {
        const response = await fetch(`/api/v1/admin/users/${userId}/permissions`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Replaces the user's overrides (absent = no override)
    const setUserPermissions = async (userId: number, overrides: PermissionSetting[]) => {
        const response = await fetch(`/api/v1/admin/users/${userId}/permissions`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ overrides }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getSearchStatus = async () => {
        const response = await fetch('/api/v1/admin/search/status', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const triggerReindex = async () => {
        const response = await fetch('/api/v1/admin/search/reindex', {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getSearchConfig = async () => {
        const response = await fetch('/api/v1/admin/search/config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateSearchConfig = async (config: { max_total_hits: number }) => {
        const response = await fetch('/api/v1/admin/search/config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    // Trash management. filters: the query params of trashFilterParams (utils/trashFilters.ts)
    const listTrash = async (
        page = 1,
        limit = 20,
        filters: Record<string, string> = {},
    ): Promise<TrashListResponse> => {
        const params = new URLSearchParams({
            ...filters,
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/trash?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Every trashed scene of a filter (ids and sizes), up to the bulk limit, for "select all"
    const listTrashIds = async (filters: Record<string, string>): Promise<TrashIdsResponse> => {
        const params = new URLSearchParams(filters);
        const response = await fetch(`/api/v1/admin/trash/ids?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getTrashFacets = async (): Promise<TrashFacets> => {
        const response = await fetch('/api/v1/admin/trash/facets', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const restoreTrashBulk = async (sceneIds: number[]): Promise<TrashBulkResult> => {
        const response = await fetch('/api/v1/admin/trash/restore-bulk', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_ids: sceneIds }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Permanent deletion of trashed scenes: their files are deleted from disk
    const deleteTrashBulk = async (sceneIds: number[]): Promise<TrashBulkResult> => {
        const response = await fetch('/api/v1/admin/trash/delete-bulk', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_ids: sceneIds }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const restoreScene = async (sceneId: number) => {
        const response = await fetch(`/api/v1/admin/trash/${sceneId}/restore`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const permanentDeleteScene = async (sceneId: number) => {
        const response = await fetch(`/api/v1/admin/trash/${sceneId}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponseWithNoContent(response);
    };

    const emptyTrash = async () => {
        const response = await fetch('/api/v1/admin/trash', {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // App settings
    const getAppSettings = async () => {
        const response = await fetch('/api/v1/admin/app-settings', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateAppSettings = async (settings: {
        serve_og_metadata: boolean;
        trash_retention_days: number;
        transcoding_enabled: boolean;
    }) => {
        const response = await fetch('/api/v1/admin/app-settings', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    // Cache of the on-the-fly conversions
    const getTranscodeCache = async (): Promise<TranscodeCacheStats> => {
        const response = await fetch('/api/v1/admin/transcode-cache', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const clearTranscodeCache = async (): Promise<TranscodeCacheClearResponse> => {
        const response = await fetch('/api/v1/admin/transcode-cache', {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchAdminUsers,
        createUser,
        updateUserRole,
        updateUserShareMarkers,
        resetUserPassword,
        deleteUser,
        fetchRoles,
        fetchPermissions,
        createRole,
        updateRole,
        deleteRole,
        setRolePermissions,
        fetchRoleEffective,
        fetchUserPermissions,
        setUserPermissions,
        getSearchStatus,
        triggerReindex,
        getSearchConfig,
        updateSearchConfig,
        listTrash,
        listTrashIds,
        getTrashFacets,
        restoreTrashBulk,
        deleteTrashBulk,
        restoreScene,
        permanentDeleteScene,
        emptyTrash,
        getAppSettings,
        updateAppSettings,
        getTranscodeCache,
        clearTranscodeCache,
    };
};
