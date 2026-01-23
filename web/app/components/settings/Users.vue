<script setup lang="ts">
import type { AdminUser, RoleResponse, PermissionResponse } from '~/types/admin';

const { fetchAdminUsers, fetchRoles, fetchPermissions, syncRolePermissions } = useApi();
const { message, error, clearMessages } = useSettingsMessage();

const usersLoading = ref(false);
const adminUsers = ref<AdminUser[]>([]);
const usersTotal = ref(0);
const usersPage = ref(1);
const usersLimit = ref(20);

const roles = ref<RoleResponse[]>([]);
const allPermissions = ref<PermissionResponse[]>([]);
const expandedRole = ref<number | null>(null);

// Modal state
const showCreateModal = ref(false);
const showEditRoleModal = ref(false);
const editRoleUser = ref<AdminUser | null>(null);
const showResetPwModal = ref(false);
const resetPwUser = ref<AdminUser | null>(null);
const showDeleteModal = ref(false);
const deleteUserTarget = ref<AdminUser | null>(null);

const loadUsers = async () => {
    usersLoading.value = true;
    clearMessages();
    try {
        const data = await fetchAdminUsers(usersPage.value, usersLimit.value);
        adminUsers.value = data.users;
        usersTotal.value = data.total;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load users';
    } finally {
        usersLoading.value = false;
    }
};

const loadRolesAndPermissions = async () => {
    try {
        const [rolesData, permsData] = await Promise.all([fetchRoles(), fetchPermissions()]);
        roles.value = rolesData.roles;
        allPermissions.value = permsData.permissions;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load roles';
    }
};

onMounted(() => {
    loadUsers();
    loadRolesAndPermissions();
});

const openEditRole = (user: AdminUser) => {
    editRoleUser.value = user;
    showEditRoleModal.value = true;
};

const openResetPassword = (user: AdminUser) => {
    resetPwUser.value = user;
    showResetPwModal.value = true;
};

const openDeleteUser = (user: AdminUser) => {
    deleteUserTarget.value = user;
    showDeleteModal.value = true;
};

const handleCreated = () => {
    showCreateModal.value = false;
    message.value = 'User created successfully';
    loadUsers();
};

const handleRoleUpdated = () => {
    showEditRoleModal.value = false;
    message.value = 'User role updated successfully';
    loadUsers();
};

const handlePasswordReset = () => {
    showResetPwModal.value = false;
    message.value = 'Password reset successfully';
};

const handleDeleted = () => {
    showDeleteModal.value = false;
    message.value = 'User deleted successfully';
    loadUsers();
};

const toggleRoleExpand = (roleId: number) => {
    expandedRole.value = expandedRole.value === roleId ? null : roleId;
};

const getRolePermissionIds = (role: RoleResponse): number[] => {
    return role.permissions.map((p) => p.id);
};

const handleTogglePermission = async (role: RoleResponse, permId: number) => {
    const currentIds = getRolePermissionIds(role);
    let newIds: number[];
    if (currentIds.includes(permId)) {
        newIds = currentIds.filter((id) => id !== permId);
    } else {
        newIds = [...currentIds, permId];
    }
    try {
        await syncRolePermissions(role.id, newIds);
        await loadRolesAndPermissions();
        message.value = 'Permissions updated';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update permissions';
    }
};

const formatDate = (dateStr: string | null): string => {
    if (!dateStr) return 'Never';
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    });
};

const roleBadgeClass = (role: string): string => {
    switch (role) {
        case 'admin':
            return 'bg-lava/15 text-lava border-lava/30';
        case 'moderator':
            return 'bg-amber-500/15 text-amber-400 border-amber-500/30';
        default:
            return 'bg-white/5 text-dim border-white/10';
    }
};
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <!-- User Management -->
        <div class="glass-panel p-5">
            <div class="mb-4 flex items-center justify-between">
                <h3 class="text-sm font-semibold text-white">User Management</h3>
                <button
                    @click="showCreateModal = true"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-[11px]
                        font-semibold text-white transition-all"
                >
                    Create User
                </button>
            </div>

            <!-- Users Table -->
            <div v-if="usersLoading" class="text-dim py-8 text-center text-xs">Loading...</div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead>
                        <tr class="text-dim border-border border-b text-[11px] uppercase tracking-wider">
                            <th class="pb-2 pr-4 font-medium">Username</th>
                            <th class="pb-2 pr-4 font-medium">Role</th>
                            <th class="pb-2 pr-4 font-medium">Created</th>
                            <th class="pb-2 pr-4 font-medium">Last Login</th>
                            <th class="pb-2 font-medium">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="u in adminUsers"
                            :key="u.id"
                            class="border-border/50 border-b last:border-0"
                        >
                            <td class="py-2.5 pr-4 text-white">{{ u.username }}</td>
                            <td class="py-2.5 pr-4">
                                <span
                                    class="inline-block rounded-full border px-2 py-0.5 text-[10px] font-medium capitalize"
                                    :class="roleBadgeClass(u.role)"
                                >
                                    {{ u.role }}
                                </span>
                            </td>
                            <td class="text-dim py-2.5 pr-4">{{ formatDate(u.created_at) }}</td>
                            <td class="text-dim py-2.5 pr-4">{{ formatDate(u.last_login_at) }}</td>
                            <td class="py-2.5">
                                <div class="flex gap-2">
                                    <button
                                        @click="openEditRole(u)"
                                        class="text-dim hover:text-white text-[11px] transition-colors"
                                    >
                                        Role
                                    </button>
                                    <button
                                        @click="openResetPassword(u)"
                                        class="text-dim hover:text-white text-[11px] transition-colors"
                                    >
                                        Password
                                    </button>
                                    <button
                                        @click="openDeleteUser(u)"
                                        class="text-lava/70 hover:text-lava text-[11px] transition-colors"
                                    >
                                        Delete
                                    </button>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <!-- Roles & Permissions -->
        <div class="glass-panel p-5">
            <h3 class="mb-4 text-sm font-semibold text-white">Roles & Permissions</h3>
            <div class="space-y-2">
                <div
                    v-for="role in roles"
                    :key="role.id"
                    class="border-border/50 rounded-lg border"
                >
                    <button
                        @click="toggleRoleExpand(role.id)"
                        class="flex w-full items-center justify-between px-3 py-2.5 text-left"
                    >
                        <div class="flex items-center gap-2">
                            <span
                                class="inline-block rounded-full border px-2 py-0.5 text-[10px] font-medium capitalize"
                                :class="roleBadgeClass(role.name)"
                            >
                                {{ role.name }}
                            </span>
                            <span class="text-dim text-[11px]">{{ role.description }}</span>
                        </div>
                        <Icon
                            :name="expandedRole === role.id ? 'lucide:chevron-up' : 'lucide:chevron-down'"
                            class="text-dim h-3.5 w-3.5"
                        />
                    </button>
                    <div
                        v-if="expandedRole === role.id"
                        class="border-border/50 border-t px-3 py-3"
                    >
                        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
                            <label
                                v-for="perm in allPermissions"
                                :key="perm.id"
                                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5
                                    transition-colors hover:bg-white/5"
                            >
                                <input
                                    type="checkbox"
                                    :checked="getRolePermissionIds(role).includes(perm.id)"
                                    @change="handleTogglePermission(role, perm.id)"
                                    class="accent-lava h-3 w-3 rounded"
                                />
                                <div>
                                    <div class="text-[11px] text-white">{{ perm.name }}</div>
                                    <div class="text-dim text-[10px]">{{ perm.description }}</div>
                                </div>
                            </label>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Modals -->
        <SettingsUserCreateModal
            :visible="showCreateModal"
            :roles="roles"
            @close="showCreateModal = false"
            @created="handleCreated"
        />
        <SettingsUserEditRoleModal
            :visible="showEditRoleModal"
            :user="editRoleUser"
            :roles="roles"
            @close="showEditRoleModal = false"
            @updated="handleRoleUpdated"
        />
        <SettingsUserResetPasswordModal
            :visible="showResetPwModal"
            :user="resetPwUser"
            @close="showResetPwModal = false"
            @reset="handlePasswordReset"
        />
        <SettingsUserDeleteModal
            :visible="showDeleteModal"
            :user="deleteUserTarget"
            @close="showDeleteModal = false"
            @deleted="handleDeleted"
        />
    </div>
</template>
