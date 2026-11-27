<script setup lang="ts">
import type { CatalogPermission, RoleSummary } from '~/types/permissions';

// Roles panel: list of the roles; create, edit, delete and permissions with roles:manage.
const props = defineProps<{
    roles: RoleSummary[];
    catalog: CatalogPermission[];
    loading: boolean;
    editable: boolean;
}>();

const emit = defineEmits<{
    message: [text: string];
    rolesChanged: [];
}>();

type ModalKind = 'form' | 'delete' | 'permissions';
const modal = ref<ModalKind | null>(null);
// Role of the modal; null in the form modal means "create"
const target = ref<RoleSummary | null>(null);

const roleNames = computed(() => new Map(props.roles.map((r) => [r.id, r.name])));

const parentNames = (role: RoleSummary): string[] =>
    role.parent_ids.map((id) => roleNames.value.get(id) ?? `#${id}`);

// The modal follows the reloaded role (permissions editor after a save)
const current = computed(() =>
    target.value ? (props.roles.find((r) => r.id === target.value!.id) ?? target.value) : null,
);

const open = (kind: ModalKind, role: RoleSummary | null) => {
    target.value = role;
    modal.value = kind;
};

const close = () => {
    modal.value = null;
};

const done = (text: string, closeModal = true) => {
    if (closeModal) close();
    emit('message', text);
    emit('rolesChanged');
};
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4 flex items-center justify-between">
            <div>
                <h3 class="text-sm font-semibold text-white">Roles & Permissions</h3>
                <p class="text-dim mt-0.5 text-[11px]">
                    A role inherits its parents' permissions (a deny from any parent wins); its own
                    settings override them.
                </p>
            </div>
            <button
                v-if="editable"
                class="bg-lava hover:bg-lava-glow shrink-0 rounded-lg px-3 py-1.5 text-[11px]
                    font-semibold text-white transition-all"
                @click="open('form', null)"
            >
                Create Role
            </button>
        </div>

        <div v-if="loading && roles.length === 0" class="text-dim py-8 text-center text-xs">
            Loading...
        </div>
        <div v-else class="space-y-2">
            <SettingsUsersRoleRow
                v-for="role in roles"
                :key="role.id"
                :role="role"
                :parents="parentNames(role)"
                :editable="editable"
                @edit="open('form', role)"
                @permissions="open('permissions', role)"
                @delete="open('delete', role)"
            />
        </div>

        <SettingsUsersRoleFormModal
            :visible="modal === 'form'"
            :role="current"
            :roles="roles"
            @close="close"
            @saved="done(current ? 'Role updated' : 'Role created')"
        />
        <SettingsUsersRoleDeleteModal
            :visible="modal === 'delete'"
            :role="current"
            @close="close"
            @deleted="done('Role deleted')"
        />
        <SettingsUsersRolePermissionsModal
            :visible="modal === 'permissions'"
            :role="current"
            :catalog="catalog"
            :editable="editable"
            @close="close"
            @saved="done(`Permissions of ${current?.name} updated`, false)"
        />
    </div>
</template>
