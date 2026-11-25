import type { MyPermissionsResponse, PermissionName } from '~/types/permissions';

/**
 * Effective permissions of the signed-in user (GET /api/v1/auth/permissions). Loaded by the
 * auth middleware before a page renders (once per user, refreshed in the background), so pages
 * can gate blocks synchronously. Until loaded, `has()` is false: gated blocks stay hidden
 * rather than flicker. The API still enforces every permission.
 */
interface PermissionsState {
    userId: number | null;
    role: string;
    superuser: boolean;
    permissions: string[];
    loaded: boolean;
}

let inflight: Promise<void> | null = null;
let inflightUserId: number | null = null;
let requestSeq = 0;

export const useMyPermissions = () => {
    const authStore = useAuthStore();
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();
    const state = useState<PermissionsState>('my-permissions', () => ({
        userId: null,
        role: '',
        superuser: false,
        permissions: [],
        loaded: false,
    }));

    const load = async (force = false): Promise<void> => {
        const userId = authStore.user?.id ?? null;
        if (userId === null) return;
        if (!force && state.value.loaded && state.value.userId === userId) return;
        if (inflight && inflightUserId === userId) return inflight;
        inflightUserId = userId;
        const seq = ++requestSeq;
        inflight = (async () => {
            try {
                const response = await fetch('/api/v1/auth/permissions', {
                    headers: getAuthHeaders(),
                    ...fetchOptions(),
                });
                const body = (await handleResponse(response)) as MyPermissionsResponse | null;
                if (authStore.user?.id !== userId) return;
                state.value = {
                    userId,
                    role: body?.role ?? authStore.user?.role ?? '',
                    superuser: body?.superuser ?? false,
                    permissions: body?.permissions ?? [],
                    loaded: true,
                };
            } catch {
                // Unknown permissions: every gated block stays hidden, the API decides anyway.
                // A background refresh keeps the last known permissions.
                if (state.value.userId !== userId) {
                    state.value = {
                        userId,
                        role: authStore.user?.role ?? '',
                        superuser: false,
                        permissions: [],
                        loaded: true,
                    };
                }
            } finally {
                if (requestSeq === seq) {
                    inflight = null;
                    inflightUserId = null;
                }
            }
        })();
        return inflight;
    };

    const current = computed(() =>
        state.value.loaded && state.value.userId === (authStore.user?.id ?? null)
            ? state.value
            : null,
    );

    const has = (permission: PermissionName): boolean => {
        const s = current.value;
        return !!s && grantsPermission(s, permission);
    };

    /** True when the user has at least one of the permissions. */
    const hasAny = (...permissions: PermissionName[]): boolean => permissions.some(has);

    return {
        load,
        has,
        hasAny,
        isSuperuser: computed(() => current.value?.superuser ?? false),
        loaded: computed(() => current.value !== null),
    };
};
