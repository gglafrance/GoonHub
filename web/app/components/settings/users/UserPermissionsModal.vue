<script setup lang="ts">
import type { AdminUser } from '~/types/admin';
import type {
    CatalogPermission,
    EffectivePermission,
    PermissionChoice,
    PermissionSetting,
    RoleSummary,
} from '~/types/permissions';

// Per-user permission overrides: Role (no override) / Allow / Deny, with the effective result.
const props = defineProps<{
    visible: boolean;
    user: AdminUser | null;
    role: RoleSummary | null;
    catalog: CatalogPermission[];
}>();

const emit = defineEmits<{
    close: [];
    saved: [];
}>();

const { fetchUserPermissions, setUserPermissions } = useApiAdmin();

const loading = ref(false);
const saving = ref(false);
const error = ref('');
const savedNote = ref(false);
const saved = ref<PermissionSetting[]>([]);
const draft = ref<PermissionSetting[]>([]);
const effective = ref<EffectivePermission[]>([]);
const roleName = ref('');

const superuser = computed(() => !!props.role?.is_superuser);
const dirty = computed(() => !sameSettings(saved.value, draft.value));

const load = async () => {
    if (!props.user) return;
    loading.value = true;
    error.value = '';
    try {
        const data = await fetchUserPermissions(props.user.id);
        saved.value = sortSettings(data?.overrides ?? []);
        draft.value = saved.value;
        effective.value = data?.effective ?? [];
        roleName.value = data?.role ?? props.user.role;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load the permissions';
    } finally {
        loading.value = false;
    }
};

watch(
    () => props.visible,
    (visible) => {
        savedNote.value = false;
        if (visible) load();
    },
    { immediate: true },
);

const handleChange = (permissionId: number, choice: PermissionChoice) => {
    draft.value = withChoice(draft.value, permissionId, choice);
    savedNote.value = false;
};

const handleSave = async () => {
    if (!props.user) return;
    saving.value = true;
    error.value = '';
    try {
        await setUserPermissions(props.user.id, draft.value);
        await load();
        savedNote.value = true;
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the permissions';
    } finally {
        saving.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4
                backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div
                class="glass-panel border-border flex max-h-[90vh] w-full max-w-2xl flex-col border"
            >
                <div class="border-border border-b px-5 py-4">
                    <h3 class="text-sm font-semibold text-white">
                        Permissions of {{ user?.username }}
                    </h3>
                    <p class="text-dim mt-1 text-[11px]">
                        Role <span class="text-white">{{ roleName || user?.role }}</span
                        >. An override replaces the role's value for this user only.
                    </p>
                </div>
                <div class="flex-1 space-y-3 overflow-y-auto px-5 py-4">
                    <div
                        v-if="error"
                        class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2
                            text-xs"
                    >
                        {{ error }}
                    </div>
                    <div
                        v-if="superuser"
                        class="border-border rounded-lg border bg-white/5 px-3 py-2 text-[11px]
                            text-white/80"
                    >
                        This user's role is a superuser role: every permission is granted and
                        overrides have no effect.
                    </div>
                    <div v-if="loading && effective.length === 0" class="text-dim py-6 text-xs">
                        Loading...
                    </div>
                    <SettingsUsersPermissionMatrix
                        v-else
                        :catalog="catalog"
                        :settings="draft"
                        :effective="effective"
                        :superuser="superuser"
                        :readonly="saving"
                        inherit-label="Role"
                        @change="handleChange"
                    />
                </div>
                <div class="border-border flex items-center justify-end gap-2 border-t px-5 py-3">
                    <span v-if="savedNote && !dirty" class="text-emerald mr-auto text-[11px]">
                        Saved
                    </span>
                    <span v-else-if="dirty" class="text-dim mr-auto text-[11px]">
                        Unsaved changes
                    </span>
                    <button
                        type="button"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white"
                        @click="handleClose"
                    >
                        Close
                    </button>
                    <button
                        v-if="!superuser"
                        type="button"
                        :disabled="saving || !dirty"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                            font-semibold text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="handleSave"
                    >
                        Save
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
