import type { CatalogPermission, RoleSummary } from '~/types/permissions';

/**
 * Roles and permission catalog of the Settings > Users tab, loaded once by the tab and shared
 * with its panels and modals (GET /admin/roles and /admin/permissions need users:manage or
 * roles:manage).
 */
export const useRbacAdmin = () => {
    const { fetchRoles, fetchPermissions } = useApiAdmin();

    const roles = ref<RoleSummary[]>([]);
    const catalog = ref<CatalogPermission[]>([]);
    const loading = ref(false);
    const error = ref('');

    const loadRoles = async () => {
        const data = await fetchRoles();
        roles.value = data?.roles ?? [];
    };

    const load = async () => {
        loading.value = true;
        error.value = '';
        try {
            const [rolesData, permsData] = await Promise.all([fetchRoles(), fetchPermissions()]);
            roles.value = rolesData?.roles ?? [];
            catalog.value = permsData?.permissions ?? [];
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to load roles';
        } finally {
            loading.value = false;
        }
    };

    const refreshRoles = async () => {
        try {
            await loadRoles();
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to load roles';
        }
    };

    return { roles, catalog, loading, error, load, refreshRoles };
};
