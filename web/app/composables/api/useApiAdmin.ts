/**
 * Admin API operations: users, roles, permissions management, trash.
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

    const fetchRoles = async () => {
        const response = await fetch('/api/v1/admin/roles', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchPermissions = async () => {
        const response = await fetch('/api/v1/admin/permissions', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const syncRolePermissions = async (roleId: number, permissionIds: number[]) => {
        const response = await fetch(`/api/v1/admin/roles/${roleId}/permissions`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ permission_ids: permissionIds }),
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

    // Trash management
    const listTrash = async (page = 1, limit = 20) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/trash?${params}`, {
            headers: getAuthHeaders(),
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

    return {
        fetchAdminUsers,
        createUser,
        updateUserRole,
        resetUserPassword,
        deleteUser,
        fetchRoles,
        fetchPermissions,
        syncRolePermissions,
        getSearchStatus,
        triggerReindex,
        getSearchConfig,
        updateSearchConfig,
        listTrash,
        restoreScene,
        permanentDeleteScene,
        emptyTrash,
    };
};
