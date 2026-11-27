<script setup lang="ts">
// Settings > Users: users (users:manage) and roles with their permissions (roles:manage).
const perms = useMyPermissions();
const { roles, catalog, loading, error: loadError, load, refreshRoles } = useRbacAdmin();
const { message, error, setMessage, setError } = useSettingsMessage();

const canManageUsers = computed(() => perms.has('users:manage'));
const canManageRoles = computed(() => perms.has('roles:manage'));

onMounted(() => {
    load();
});

// A change of roles, role permissions or overrides may change the signed-in user's own
// permissions: refresh them too
const onRolesChanged = () => {
    void refreshRoles();
    void perms.load(true);
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
            v-if="error || loadError"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error || loadError }}
        </div>

        <SettingsUsersUserTable
            v-if="canManageUsers"
            :roles="roles"
            :catalog="catalog"
            @message="setMessage"
            @error="setError"
            @roles-changed="onRolesChanged"
        />

        <SettingsUsersRolesPanel
            :roles="roles"
            :catalog="catalog"
            :loading="loading"
            :editable="canManageRoles"
            @message="setMessage"
            @roles-changed="onRolesChanged"
        />
    </div>
</template>
