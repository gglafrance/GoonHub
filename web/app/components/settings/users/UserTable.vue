<script setup lang="ts">
import type { AdminUser } from '~/types/admin';
import type { CatalogPermission, RoleSummary } from '~/types/permissions';

// Users panel (users:manage): the users table and its modals.
const props = defineProps<{
    roles: RoleSummary[];
    catalog: CatalogPermission[];
}>();

const emit = defineEmits<{
    message: [text: string];
    error: [text: string];
    rolesChanged: [];
}>();

const { fetchAdminUsers, updateUserShareMarkers } = useApiAdmin();

const loading = ref(false);
const users = ref<AdminUser[]>([]);
const page = 1;
const limit = 20;

type ModalKind = 'create' | 'role' | 'permissions' | 'password' | 'delete';
const modal = ref<ModalKind | null>(null);
const target = ref<AdminUser | null>(null);

const roleByName = computed(() => new Map(props.roles.map((r) => [r.name, r])));

const loadUsers = async () => {
    loading.value = true;
    try {
        const data = await fetchAdminUsers(page, limit);
        users.value = data?.users ?? [];
    } catch (e: unknown) {
        emit('error', e instanceof Error ? e.message : 'Failed to load users');
    } finally {
        loading.value = false;
    }
};

onMounted(loadUsers);

const open = (kind: ModalKind, user: AdminUser | null = null) => {
    target.value = user;
    modal.value = kind;
};

const close = () => {
    modal.value = null;
};

// Done: closes the modal, reports and reloads what changed
const done = (text: string, reloadUsers = true, rolesChanged = false) => {
    close();
    emit('message', text);
    if (reloadUsers) loadUsers();
    if (rolesChanged) emit('rolesChanged');
};

// Users whose marker sharing is being saved (toggle disabled meanwhile)
const shareSaving = ref<Set<number>>(new Set());

const handleShareMarkers = async (user: AdminUser, share: boolean) => {
    shareSaving.value = new Set(shareSaving.value).add(user.id);
    try {
        await updateUserShareMarkers(user.id, share);
        user.share_markers = share;
        emit(
            'message',
            share
                ? `${user.username}'s markers are now visible to every user`
                : `${user.username}'s markers are private again`,
        );
    } catch (e: unknown) {
        emit('error', e instanceof Error ? e.message : 'Failed to update marker sharing');
    } finally {
        const next = new Set(shareSaving.value);
        next.delete(user.id);
        shareSaving.value = next;
    }
};
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-white">User Management</h3>
            <button
                class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-[11px] font-semibold
                    text-white transition-all"
                @click="open('create')"
            >
                Create User
            </button>
        </div>

        <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>
        <div v-else class="overflow-x-auto">
            <table class="w-full text-left text-xs">
                <thead>
                    <tr
                        class="text-dim border-border border-b text-[11px] tracking-wider uppercase"
                    >
                        <th class="pr-4 pb-2 font-medium">Username</th>
                        <th class="pr-4 pb-2 font-medium">Role</th>
                        <th
                            class="pr-4 pb-2 font-medium"
                            title="Markers visible (read-only) to every user"
                        >
                            Shared markers
                        </th>
                        <th class="pr-4 pb-2 font-medium">Created</th>
                        <th class="pr-4 pb-2 font-medium">Last Login</th>
                        <th class="pb-2 font-medium">Actions</th>
                    </tr>
                </thead>
                <tbody>
                    <SettingsUsersUserRow
                        v-for="u in users"
                        :key="u.id"
                        :user="u"
                        :role="roleByName.get(u.role) ?? null"
                        :share-saving="shareSaving.has(u.id)"
                        @share-markers="handleShareMarkers(u, $event)"
                        @edit-role="open('role', u)"
                        @permissions="open('permissions', u)"
                        @reset-password="open('password', u)"
                        @delete="open('delete', u)"
                    />
                </tbody>
            </table>
        </div>

        <SettingsUserCreateModal
            :visible="modal === 'create'"
            :roles="roles"
            @close="close"
            @created="done('User created successfully', true, true)"
        />
        <SettingsUserEditRoleModal
            :visible="modal === 'role'"
            :user="target"
            :roles="roles"
            @close="close"
            @updated="done('User role updated successfully', true, true)"
        />
        <SettingsUsersUserPermissionsModal
            :visible="modal === 'permissions'"
            :user="target"
            :role="target ? (roleByName.get(target.role) ?? null) : null"
            :catalog="catalog"
            @close="close"
            @saved="emit('message', `${target?.username}'s permissions updated`)"
        />
        <SettingsUserResetPasswordModal
            :visible="modal === 'password'"
            :user="target"
            @close="close"
            @reset="done('Password reset successfully', false)"
        />
        <SettingsUserDeleteModal
            :visible="modal === 'delete'"
            :user="target"
            @close="close"
            @deleted="done('User deleted successfully', true, true)"
        />
    </div>
</template>
