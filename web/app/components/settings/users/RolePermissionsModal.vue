<script setup lang="ts">
import type {
    CatalogPermission,
    EffectivePermission,
    PermissionChoice,
    PermissionSetting,
    RoleSummary,
} from '~/types/permissions';

// Permission editor of a role: each change is saved right away, then the effective values
// (own setting, inherited from a parent) are reloaded.
const props = defineProps<{
    visible: boolean;
    role: RoleSummary | null;
    catalog: CatalogPermission[];
    // roles:manage: edit and see the effective values (the API needs it for both)
    editable: boolean;
}>();

const emit = defineEmits<{
    close: [];
    saved: [];
}>();

const { setRolePermissions, fetchRoleEffective } = useApiAdmin();

const settings = ref<PermissionSetting[]>([]);
const effective = ref<EffectivePermission[]>([]);
const saving = ref(false);
const error = ref('');

const superuser = computed(() => !!props.role?.is_superuser);

const loadEffective = async () => {
    if (!props.role || !props.editable || superuser.value) return;
    try {
        const data = await fetchRoleEffective(props.role.id);
        effective.value = data?.permissions ?? [];
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load the effective permissions';
    }
};

watch(
    () => [props.visible, props.role?.id] as const,
    ([visible]) => {
        if (!visible || !props.role) return;
        error.value = '';
        effective.value = [];
        settings.value = sortSettings(props.role.settings);
        loadEffective();
    },
    { immediate: true },
);

const handleChange = async (permissionId: number, choice: PermissionChoice) => {
    if (!props.role || saving.value) return;
    const previous = settings.value;
    settings.value = withChoice(previous, permissionId, choice);
    saving.value = true;
    error.value = '';
    try {
        await setRolePermissions(props.role.id, settings.value);
        await loadEffective();
        emit('saved');
    } catch (e: unknown) {
        settings.value = previous;
        error.value = e instanceof Error ? e.message : 'Failed to save the permissions';
    } finally {
        saving.value = false;
    }
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4
                backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div
                class="glass-panel border-border flex max-h-[90vh] w-full max-w-2xl flex-col border"
            >
                <div class="border-border flex items-start justify-between border-b px-5 py-4">
                    <div>
                        <h3 class="text-sm font-semibold text-white">
                            Permissions of {{ role?.name }}
                        </h3>
                        <p class="text-dim mt-1 text-[11px]">
                            Inherit takes the parents' value; Allow and Deny override it. Changes
                            are saved right away.
                        </p>
                    </div>
                    <Icon
                        v-if="saving"
                        name="svg-spinners:90-ring-with-bg"
                        class="text-dim h-4 w-4"
                    />
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
                        Superuser role: every permission is granted and none can be denied.
                    </div>
                    <SettingsUsersPermissionMatrix
                        :catalog="catalog"
                        :settings="settings"
                        :effective="effective"
                        :superuser="superuser"
                        :readonly="!editable || saving"
                        @change="handleChange"
                    />
                </div>
                <div class="border-border flex justify-end border-t px-5 py-3">
                    <button
                        type="button"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white"
                        @click="emit('close')"
                    >
                        Close
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
