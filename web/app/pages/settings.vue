<script setup lang="ts">
import type { SortOrder } from '~/types/settings';
import type { AdminUser, RoleResponse, PermissionResponse } from '~/types/admin';

const settingsStore = useSettingsStore();
const authStore = useAuthStore();
const {
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
} = useApi();

type TabType = 'account' | 'player' | 'app' | 'users';
const activeTab = ref<TabType>('account');

const availableTabs = computed(() => {
    const tabs: TabType[] = ['account', 'player', 'app'];
    if (authStore.user?.role === 'admin') {
        tabs.push('users');
    }
    return tabs;
});

// Account tab state
const newUsername = ref('');
const currentPassword = ref('');
const newPassword = ref('');
const confirmPassword = ref('');
const accountMessage = ref('');
const accountError = ref('');
const accountLoading = ref(false);

// Player tab state
const playerAutoplay = ref(false);
const playerVolume = ref(100);
const playerLoop = ref(false);
const playerMessage = ref('');
const playerError = ref('');

// App tab state
const appVideosPerPage = ref(20);
const appSortOrder = ref<SortOrder>('created_at_desc');
const appMessage = ref('');
const appError = ref('');

// Users tab state
const usersLoading = ref(false);
const usersError = ref('');
const usersMessage = ref('');
const adminUsers = ref<AdminUser[]>([]);
const usersTotal = ref(0);
const usersPage = ref(1);
const usersLimit = ref(20);

// Roles & permissions
const roles = ref<RoleResponse[]>([]);
const allPermissions = ref<PermissionResponse[]>([]);
const expandedRole = ref<number | null>(null);

// Create user modal
const showCreateModal = ref(false);
const createUsername = ref('');
const createPassword = ref('');
const createRole = ref('user');
const createLoading = ref(false);

// Edit role modal
const showEditRoleModal = ref(false);
const editRoleUser = ref<AdminUser | null>(null);
const editRoleValue = ref('');
const editRoleLoading = ref(false);

// Reset password modal
const showResetPwModal = ref(false);
const resetPwUser = ref<AdminUser | null>(null);
const resetPwValue = ref('');
const resetPwLoading = ref(false);

// Delete confirmation modal
const showDeleteModal = ref(false);
const deleteUserTarget = ref<AdminUser | null>(null);
const deleteLoading = ref(false);

const sortOptions: { value: SortOrder; label: string }[] = [
    { value: 'created_at_desc', label: 'Newest First' },
    { value: 'created_at_asc', label: 'Oldest First' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest First' },
    { value: 'duration_desc', label: 'Longest First' },
    { value: 'size_asc', label: 'Smallest First' },
    { value: 'size_desc', label: 'Largest First' },
];

// Initialize form values from store
const initFormValues = () => {
    playerAutoplay.value = settingsStore.autoplay;
    playerVolume.value = settingsStore.defaultVolume;
    playerLoop.value = settingsStore.loop;
    appVideosPerPage.value = settingsStore.videosPerPage;
    appSortOrder.value = settingsStore.defaultSortOrder;
};

onMounted(async () => {
    await settingsStore.loadSettings();
    initFormValues();
});

watch(
    () => settingsStore.settings,
    () => {
        initFormValues();
    },
);

const handleChangeUsername = async () => {
    accountMessage.value = '';
    accountError.value = '';
    if (!newUsername.value || newUsername.value.length < 3) {
        accountError.value = 'Username must be at least 3 characters';
        return;
    }
    accountLoading.value = true;
    try {
        await changeUsername(newUsername.value);
        accountMessage.value = 'Username changed successfully';
        authStore.user!.username = newUsername.value;
        newUsername.value = '';
    } catch (e: unknown) {
        accountError.value = e instanceof Error ? e.message : 'Failed to change username';
    } finally {
        accountLoading.value = false;
    }
};

const handleChangePassword = async () => {
    accountMessage.value = '';
    accountError.value = '';
    if (!currentPassword.value || !newPassword.value) {
        accountError.value = 'All password fields are required';
        return;
    }
    if (newPassword.value !== confirmPassword.value) {
        accountError.value = 'New passwords do not match';
        return;
    }
    if (newPassword.value.length < 6) {
        accountError.value = 'New password must be at least 6 characters';
        return;
    }
    accountLoading.value = true;
    try {
        await changePassword(currentPassword.value, newPassword.value);
        accountMessage.value = 'Password changed successfully';
        currentPassword.value = '';
        newPassword.value = '';
        confirmPassword.value = '';
    } catch (e: unknown) {
        accountError.value = e instanceof Error ? e.message : 'Failed to change password';
    } finally {
        accountLoading.value = false;
    }
};

const handleSavePlayer = async () => {
    playerMessage.value = '';
    playerError.value = '';
    try {
        await settingsStore.updatePlayer(
            playerAutoplay.value,
            playerVolume.value,
            playerLoop.value,
        );
        playerMessage.value = 'Player settings saved';
    } catch (e: unknown) {
        playerError.value = e instanceof Error ? e.message : 'Failed to save settings';
    }
};

const handleSaveApp = async () => {
    appMessage.value = '';
    appError.value = '';
    try {
        await settingsStore.updateApp(appVideosPerPage.value, appSortOrder.value);
        appMessage.value = 'App settings saved';
    } catch (e: unknown) {
        appError.value = e instanceof Error ? e.message : 'Failed to save settings';
    }
};

// Users tab methods
const loadUsers = async () => {
    usersLoading.value = true;
    usersError.value = '';
    try {
        const data = await fetchAdminUsers(usersPage.value, usersLimit.value);
        adminUsers.value = data.users;
        usersTotal.value = data.total;
    } catch (e: unknown) {
        usersError.value = e instanceof Error ? e.message : 'Failed to load users';
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
        usersError.value = e instanceof Error ? e.message : 'Failed to load roles';
    }
};

watch(activeTab, (tab) => {
    if (tab === 'users') {
        loadUsers();
        loadRolesAndPermissions();
    }
});

const handleCreateUser = async () => {
    createLoading.value = true;
    usersMessage.value = '';
    usersError.value = '';
    try {
        await createUser(createUsername.value, createPassword.value, createRole.value);
        usersMessage.value = 'User created successfully';
        showCreateModal.value = false;
        createUsername.value = '';
        createPassword.value = '';
        createRole.value = 'user';
        await loadUsers();
    } catch (e: unknown) {
        usersError.value = e instanceof Error ? e.message : 'Failed to create user';
    } finally {
        createLoading.value = false;
    }
};

const openEditRole = (user: AdminUser) => {
    editRoleUser.value = user;
    editRoleValue.value = user.role;
    showEditRoleModal.value = true;
};

const handleEditRole = async () => {
    if (!editRoleUser.value) return;
    editRoleLoading.value = true;
    usersMessage.value = '';
    usersError.value = '';
    try {
        await updateUserRole(editRoleUser.value.id, editRoleValue.value);
        usersMessage.value = 'User role updated successfully';
        showEditRoleModal.value = false;
        await loadUsers();
    } catch (e: unknown) {
        usersError.value = e instanceof Error ? e.message : 'Failed to update role';
    } finally {
        editRoleLoading.value = false;
    }
};

const openResetPassword = (user: AdminUser) => {
    resetPwUser.value = user;
    resetPwValue.value = '';
    showResetPwModal.value = true;
};

const handleResetPassword = async () => {
    if (!resetPwUser.value) return;
    if (resetPwValue.value.length < 6) {
        usersError.value = 'Password must be at least 6 characters';
        return;
    }
    resetPwLoading.value = true;
    usersMessage.value = '';
    usersError.value = '';
    try {
        await resetUserPassword(resetPwUser.value.id, resetPwValue.value);
        usersMessage.value = 'Password reset successfully';
        showResetPwModal.value = false;
    } catch (e: unknown) {
        usersError.value = e instanceof Error ? e.message : 'Failed to reset password';
    } finally {
        resetPwLoading.value = false;
    }
};

const openDeleteUser = (user: AdminUser) => {
    deleteUserTarget.value = user;
    showDeleteModal.value = true;
};

const handleDeleteUser = async () => {
    if (!deleteUserTarget.value) return;
    deleteLoading.value = true;
    usersMessage.value = '';
    usersError.value = '';
    try {
        await deleteUser(deleteUserTarget.value.id);
        usersMessage.value = 'User deleted successfully';
        showDeleteModal.value = false;
        await loadUsers();
    } catch (e: unknown) {
        usersError.value = e instanceof Error ? e.message : 'Failed to delete user';
    } finally {
        deleteLoading.value = false;
    }
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
        usersMessage.value = 'Permissions updated';
    } catch (e: unknown) {
        usersError.value = e instanceof Error ? e.message : 'Failed to update permissions';
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

definePageMeta({
    title: 'Settings - GoonHub',
});
</script>

<template>
    <div class="mx-auto max-w-2xl px-4 py-8 sm:px-5">
        <h1 class="mb-6 text-lg font-bold tracking-tight text-white">Settings</h1>

        <!-- Tabs -->
        <div class="border-border mb-6 flex gap-1 border-b pb-px">
            <button
                v-for="tab in availableTabs"
                :key="tab"
                @click="activeTab = tab"
                class="relative px-4 py-2 text-xs font-medium capitalize transition-colors"
                :class="activeTab === tab ? 'text-lava' : 'text-dim hover:text-white'"
            >
                {{ tab }}
                <div
                    v-if="activeTab === tab"
                    class="bg-lava absolute right-0 bottom-0 left-0 h-0.5 rounded-full"
                ></div>
            </button>
        </div>

        <!-- Account Tab -->
        <div v-if="activeTab === 'account'" class="space-y-6">
            <!-- Success/Error Messages -->
            <div
                v-if="accountMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ accountMessage }}
            </div>
            <div
                v-if="accountError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ accountError }}
            </div>

            <!-- Username Change -->
            <div class="glass-panel p-5">
                <h3 class="mb-4 text-sm font-semibold text-white">Change Username</h3>
                <form @submit.prevent="handleChangeUsername" class="space-y-3">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            New Username
                        </label>
                        <input
                            v-model="newUsername"
                            type="text"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter new username"
                        />
                    </div>
                    <button
                        type="submit"
                        :disabled="accountLoading || !newUsername"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Update Username
                    </button>
                </form>
            </div>

            <!-- Password Change -->
            <div class="glass-panel p-5">
                <h3 class="mb-4 text-sm font-semibold text-white">Change Password</h3>
                <form @submit.prevent="handleChangePassword" class="space-y-3">
                    <input type="text" :value="authStore.user?.username" autocomplete="username" class="hidden" aria-hidden="true" tabindex="-1" />
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Current Password
                        </label>
                        <input
                            v-model="currentPassword"
                            type="password"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter current password"
                            autocomplete="current-password"
                        />
                    </div>
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            New Password
                        </label>
                        <input
                            v-model="newPassword"
                            type="password"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter new password"
                            autocomplete="new-password"
                        />
                    </div>
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Confirm New Password
                        </label>
                        <input
                            v-model="confirmPassword"
                            type="password"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Confirm new password"
                            autocomplete="new-password"
                        />
                    </div>
                    <button
                        type="submit"
                        :disabled="
                            accountLoading || !currentPassword || !newPassword || !confirmPassword
                        "
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Update Password
                    </button>
                </form>
            </div>
        </div>

        <!-- Player Tab -->
        <div v-if="activeTab === 'player'" class="space-y-6">
            <div
                v-if="playerMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ playerMessage }}
            </div>
            <div
                v-if="playerError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ playerError }}
            </div>

            <div class="glass-panel p-5">
                <h3 class="mb-5 text-sm font-semibold text-white">Player Preferences</h3>
                <div class="space-y-5">
                    <!-- Autoplay Toggle -->
                    <div class="flex items-center justify-between">
                        <div>
                            <div class="text-sm text-white">Autoplay</div>
                            <div class="text-dim text-xs">
                                Automatically play videos when opened
                            </div>
                        </div>
                        <button
                            @click="playerAutoplay = !playerAutoplay"
                            class="relative h-5 w-9 rounded-full transition-colors"
                            :class="playerAutoplay ? 'bg-emerald' : 'bg-border'"
                        >
                            <div
                                class="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow
                                    transition-transform"
                                :class="playerAutoplay ? 'translate-x-4' : 'translate-x-0.5'"
                            ></div>
                        </button>
                    </div>

                    <!-- Loop Toggle -->
                    <div class="flex items-center justify-between">
                        <div>
                            <div class="text-sm text-white">Loop</div>
                            <div class="text-dim text-xs">Loop videos when they finish</div>
                        </div>
                        <button
                            @click="playerLoop = !playerLoop"
                            class="relative h-5 w-9 rounded-full transition-colors"
                            :class="playerLoop ? 'bg-emerald' : 'bg-border'"
                        >
                            <div
                                class="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow
                                    transition-transform"
                                :class="playerLoop ? 'translate-x-4' : 'translate-x-0.5'"
                            ></div>
                        </button>
                    </div>

                    <!-- Volume Slider -->
                    <div>
                        <div class="mb-2 flex items-center justify-between">
                            <div>
                                <div class="text-sm text-white">Default Volume</div>
                                <div class="text-dim text-xs">Initial volume level for videos</div>
                            </div>
                            <span class="text-dim font-mono text-xs">{{ playerVolume }}%</span>
                        </div>
                        <input
                            v-model.number="playerVolume"
                            type="range"
                            min="0"
                            max="100"
                            step="5"
                            class="accent-lava w-full"
                        />
                    </div>

                    <button
                        @click="handleSavePlayer"
                        :disabled="settingsStore.isLoading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Save Player Settings
                    </button>
                </div>
            </div>
        </div>

        <!-- App Tab -->
        <div v-if="activeTab === 'app'" class="space-y-6">
            <div
                v-if="appMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ appMessage }}
            </div>
            <div
                v-if="appError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ appError }}
            </div>

            <div class="glass-panel p-5">
                <h3 class="mb-5 text-sm font-semibold text-white">App Preferences</h3>
                <div class="space-y-5">
                    <!-- Videos Per Page -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Videos Per Page
                        </label>
                        <input
                            v-model.number="appVideosPerPage"
                            type="number"
                            min="1"
                            max="100"
                            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                w-full max-w-32 rounded-lg border px-3.5 py-2.5 text-sm text-white
                                transition-all focus:ring-1 focus:outline-none"
                        />
                    </div>

                    <!-- Sort Order -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Default Sort Order
                        </label>
                        <select
                            v-model="appSortOrder"
                            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                w-full max-w-64 appearance-none rounded-lg border px-3.5 py-2.5
                                text-sm text-white transition-all focus:ring-1 focus:outline-none"
                        >
                            <option
                                v-for="opt in sortOptions"
                                :key="opt.value"
                                :value="opt.value"
                                class="bg-panel"
                            >
                                {{ opt.label }}
                            </option>
                        </select>
                    </div>

                    <button
                        @click="handleSaveApp"
                        :disabled="settingsStore.isLoading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Save App Settings
                    </button>
                </div>
            </div>
        </div>

        <!-- Users Tab (Admin Only) -->
        <div v-if="activeTab === 'users'" class="space-y-6">
            <div
                v-if="usersMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ usersMessage }}
            </div>
            <div
                v-if="usersError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ usersError }}
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
        </div>

        <!-- Create User Modal -->
        <Teleport to="body">
            <div
                v-if="showCreateModal"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
                @click.self="showCreateModal = false"
            >
                <div class="glass-panel border-border w-full max-w-sm border p-6">
                    <h3 class="mb-4 text-sm font-semibold text-white">Create User</h3>
                    <form @submit.prevent="handleCreateUser" class="space-y-3">
                        <div>
                            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                                Username
                            </label>
                            <input
                                v-model="createUsername"
                                type="text"
                                autocomplete="username"
                                class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5
                                    text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                                placeholder="Username"
                            />
                        </div>
                        <div>
                            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                                Password
                            </label>
                            <input
                                v-model="createPassword"
                                type="password"
                                autocomplete="new-password"
                                class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5
                                    text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                                placeholder="Password (min 6 chars)"
                            />
                        </div>
                        <div>
                            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                                Role
                            </label>
                            <select
                                v-model="createRole"
                                class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                    w-full appearance-none rounded-lg border px-3.5 py-2.5 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            >
                                <option v-for="r in roles" :key="r.id" :value="r.name" class="bg-panel">
                                    {{ r.name }}
                                </option>
                            </select>
                        </div>
                        <div class="flex justify-end gap-2 pt-2">
                            <button
                                type="button"
                                @click="showCreateModal = false"
                                class="text-dim hover:text-white rounded-lg px-3 py-1.5 text-xs transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                :disabled="createLoading || !createUsername || !createPassword"
                                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                    font-semibold text-white transition-all
                                    disabled:cursor-not-allowed disabled:opacity-40"
                            >
                                Create
                            </button>
                        </div>
                    </form>
                </div>
            </div>
        </Teleport>

        <!-- Edit Role Modal -->
        <Teleport to="body">
            <div
                v-if="showEditRoleModal"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
                @click.self="showEditRoleModal = false"
            >
                <div class="glass-panel border-border w-full max-w-sm border p-6">
                    <h3 class="mb-4 text-sm font-semibold text-white">
                        Change Role for {{ editRoleUser?.username }}
                    </h3>
                    <form @submit.prevent="handleEditRole" class="space-y-3">
                        <div>
                            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                                Role
                            </label>
                            <select
                                v-model="editRoleValue"
                                class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                    w-full appearance-none rounded-lg border px-3.5 py-2.5 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            >
                                <option v-for="r in roles" :key="r.id" :value="r.name" class="bg-panel">
                                    {{ r.name }}
                                </option>
                            </select>
                        </div>
                        <div class="flex justify-end gap-2 pt-2">
                            <button
                                type="button"
                                @click="showEditRoleModal = false"
                                class="text-dim hover:text-white rounded-lg px-3 py-1.5 text-xs transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                :disabled="editRoleLoading"
                                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                    font-semibold text-white transition-all
                                    disabled:cursor-not-allowed disabled:opacity-40"
                            >
                                Update
                            </button>
                        </div>
                    </form>
                </div>
            </div>
        </Teleport>

        <!-- Reset Password Modal -->
        <Teleport to="body">
            <div
                v-if="showResetPwModal"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
                @click.self="showResetPwModal = false"
            >
                <div class="glass-panel border-border w-full max-w-sm border p-6">
                    <h3 class="mb-4 text-sm font-semibold text-white">
                        Reset Password for {{ resetPwUser?.username }}
                    </h3>
                    <form @submit.prevent="handleResetPassword" class="space-y-3">
                        <input type="text" :value="resetPwUser?.username" autocomplete="username" class="hidden" aria-hidden="true" tabindex="-1" />
                        <div>
                            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                                New Password
                            </label>
                            <input
                                v-model="resetPwValue"
                                type="password"
                                autocomplete="new-password"
                                class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5
                                    text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                                placeholder="New password (min 6 chars)"
                            />
                        </div>
                        <div class="flex justify-end gap-2 pt-2">
                            <button
                                type="button"
                                @click="showResetPwModal = false"
                                class="text-dim hover:text-white rounded-lg px-3 py-1.5 text-xs transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                :disabled="resetPwLoading || !resetPwValue"
                                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                    font-semibold text-white transition-all
                                    disabled:cursor-not-allowed disabled:opacity-40"
                            >
                                Reset
                            </button>
                        </div>
                    </form>
                </div>
            </div>
        </Teleport>

        <!-- Delete Confirmation Modal -->
        <Teleport to="body">
            <div
                v-if="showDeleteModal"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
                @click.self="showDeleteModal = false"
            >
                <div class="glass-panel border-border w-full max-w-sm border p-6">
                    <h3 class="mb-2 text-sm font-semibold text-white">Delete User</h3>
                    <p class="text-dim mb-4 text-xs">
                        Are you sure you want to delete
                        <span class="text-white">{{ deleteUserTarget?.username }}</span>? This action cannot be undone.
                    </p>
                    <div class="flex justify-end gap-2">
                        <button
                            @click="showDeleteModal = false"
                            class="text-dim hover:text-white rounded-lg px-3 py-1.5 text-xs transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            @click="handleDeleteUser"
                            :disabled="deleteLoading"
                            class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold
                                text-white transition-all hover:bg-red-500
                                disabled:cursor-not-allowed disabled:opacity-40"
                        >
                            Delete
                        </button>
                    </div>
                </div>
            </div>
        </Teleport>
    </div>
</template>
