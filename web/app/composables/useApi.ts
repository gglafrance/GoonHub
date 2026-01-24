export const useApi = () => {
    const authStore = useAuthStore();

    const getAuthHeaders = () => {
        const headers: Record<string, string> = {
            'Content-Type': 'application/json',
        };

        if (authStore.token) {
            headers['Authorization'] = `Bearer ${authStore.token}`;
        }

        return headers;
    };

    const handleResponse = async (response: Response) => {
        if (response.status === 401) {
            authStore.logout();
            throw new Error('Unauthorized');
        }

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }

        return response.json();
    };

    const uploadVideo = async (file: File, title?: string) => {
        const formData = new FormData();
        formData.append('video', file);
        if (title) {
            formData.append('title', title);
        }

        const headers: Record<string, string> = {};
        if (authStore.token) {
            headers['Authorization'] = `Bearer ${authStore.token}`;
        }

        const response = await fetch('/api/v1/videos', {
            method: 'POST',
            body: formData,
            headers,
        });

        return handleResponse(response);
    };

    const fetchVideos = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });

        const response = await fetch(`/api/v1/videos?${params}`, {
            headers: getAuthHeaders(),
        });

        return handleResponse(response);
    };

    const fetchVideo = async (id: number) => {
        const response = await fetch(`/api/v1/videos/${id}`, {
            headers: getAuthHeaders(),
        });

        return handleResponse(response);
    };

    const fetchSettings = async () => {
        const response = await fetch('/api/v1/settings', {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const updatePlayerSettings = async (settings: {
        autoplay: boolean;
        default_volume: number;
        loop: boolean;
    }) => {
        const response = await fetch('/api/v1/settings/player', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const updateAppSettings = async (settings: {
        videos_per_page: number;
        default_sort_order: string;
    }) => {
        const response = await fetch('/api/v1/settings/app', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const updateTagSettings = async (settings: { default_tag_sort: string }) => {
        const response = await fetch('/api/v1/settings/tags', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const changePassword = async (currentPassword: string, newPassword: string) => {
        const response = await fetch('/api/v1/settings/password', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
        });
        return handleResponse(response);
    };

    const changeUsername = async (username: string) => {
        const response = await fetch('/api/v1/settings/username', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ username }),
        });
        return handleResponse(response);
    };

    const fetchAdminUsers = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/users?${params}`, {
            headers: getAuthHeaders(),
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
        });
        return handleResponse(response);
    };

    const fetchRoles = async () => {
        const response = await fetch('/api/v1/admin/roles', {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const fetchPermissions = async () => {
        const response = await fetch('/api/v1/admin/permissions', {
            headers: getAuthHeaders(),
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

    const fetchJobs = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/jobs?${params}`, {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const fetchPoolConfig = async () => {
        const response = await fetch('/api/v1/admin/pool-config', {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const updatePoolConfig = async (config: {
        metadata_workers: number;
        thumbnail_workers: number;
        sprites_workers: number;
    }) => {
        const response = await fetch('/api/v1/admin/pool-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const fetchProcessingConfig = async () => {
        const response = await fetch('/api/v1/admin/processing-config', {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const updateProcessingConfig = async (config: {
        max_frame_dimension_sm: number;
        max_frame_dimension_lg: number;
        frame_quality_sm: number;
        frame_quality_lg: number;
        frame_quality_sprites: number;
        sprites_concurrency: number;
    }) => {
        const response = await fetch('/api/v1/admin/processing-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const fetchTriggerConfig = async () => {
        const response = await fetch('/api/v1/admin/trigger-config', {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const updateTriggerConfig = async (config: {
        phase: string;
        trigger_type: string;
        after_phase?: string | null;
        cron_expression?: string | null;
    }) => {
        const response = await fetch('/api/v1/admin/trigger-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const triggerVideoPhase = async (videoId: number, phase: string) => {
        const response = await fetch(`/api/v1/admin/videos/${videoId}/process/${phase}`, {
            method: 'POST',
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const extractThumbnail = async (videoId: number, timecode: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/thumbnail`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ timecode }),
        });
        return handleResponse(response);
    };

    const uploadThumbnail = async (videoId: number, file: File) => {
        const formData = new FormData();
        formData.append('thumbnail', file);

        const headers: Record<string, string> = {};
        if (authStore.token) {
            headers['Authorization'] = `Bearer ${authStore.token}`;
        }

        const response = await fetch(`/api/v1/videos/${videoId}/thumbnail/upload`, {
            method: 'POST',
            body: formData,
            headers,
        });
        return handleResponse(response);
    };

    const fetchTags = async () => {
        const response = await fetch('/api/v1/tags', {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const createTag = async (name: string, color: string) => {
        const response = await fetch('/api/v1/tags', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, color }),
        });
        return handleResponse(response);
    };

    const deleteTag = async (id: number) => {
        const response = await fetch(`/api/v1/tags/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const fetchVideoTags = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/tags`, {
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const setVideoTags = async (videoId: number, tagIds: number[]) => {
        const response = await fetch(`/api/v1/videos/${videoId}/tags`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ tag_ids: tagIds }),
        });
        return handleResponse(response);
    };

    return {
        uploadVideo,
        fetchVideos,
        fetchVideo,
        fetchSettings,
        updatePlayerSettings,
        updateAppSettings,
        updateTagSettings,
        changePassword,
        changeUsername,
        fetchAdminUsers,
        createUser,
        updateUserRole,
        resetUserPassword,
        deleteUser,
        fetchRoles,
        fetchPermissions,
        syncRolePermissions,
        fetchJobs,
        fetchPoolConfig,
        updatePoolConfig,
        fetchProcessingConfig,
        updateProcessingConfig,
        fetchTriggerConfig,
        updateTriggerConfig,
        triggerVideoPhase,
        extractThumbnail,
        uploadThumbnail,
        fetchTags,
        createTag,
        deleteTag,
        fetchVideoTags,
        setVideoTags,
    };
};
