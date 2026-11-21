/**
 * Permissions of the signed-in user's role (GET /api/v1/auth/permissions).
 * Loaded once per user and shared between components. Used to hide or disable actions the
 * user cannot perform; the API still enforces every permission.
 */
interface PermissionsState {
    userId: number | null;
    permissions: string[];
    loaded: boolean;
}

let inflight: Promise<void> | null = null;

export const useMyPermissions = () => {
    const authStore = useAuthStore();
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();
    const state = useState<PermissionsState>('my-permissions', () => ({
        userId: null,
        permissions: [],
        loaded: false,
    }));

    const load = async (force = false) => {
        const userId = authStore.user?.id ?? null;
        if (userId === null) return;
        if (!force && state.value.loaded && state.value.userId === userId) return;
        if (inflight) return inflight;
        inflight = (async () => {
            try {
                const response = await fetch('/api/v1/auth/permissions', {
                    headers: getAuthHeaders(),
                    ...fetchOptions(),
                });
                const body: { permissions: string[] } = await handleResponse(response);
                state.value = { userId, permissions: body.permissions || [], loaded: true };
            } catch {
                // Unknown permissions: everything stays disabled except role-based checks
                state.value = { userId, permissions: [], loaded: true };
            } finally {
                inflight = null;
            }
        })();
        return inflight;
    };

    const isAdmin = computed(() => authStore.user?.role === 'admin');
    // No admin shortcut: the API checks permissions for every role, admin included
    const has = (permission: string) => state.value.permissions.includes(permission);

    return {
        load,
        has,
        isAdmin,
        loaded: computed(() => state.value.loaded),
    };
};
