<script setup lang="ts">
import type { RoleSummary } from '~/types/permissions';

// Create a role (role null) or edit its name, description and parents.
const props = defineProps<{
    visible: boolean;
    role: RoleSummary | null;
    roles: RoleSummary[];
}>();

const emit = defineEmits<{
    close: [];
    saved: [];
}>();

const { createRole, updateRole } = useApiAdmin();

const name = ref('');
const description = ref('');
const parentIds = ref<number[]>([]);
const loading = ref(false);
const error = ref('');

const editing = computed(() => props.role !== null);
// The superuser role cannot be renamed
const nameLocked = computed(() => !!props.role?.is_superuser);
const candidates = computed(() => parentCandidates(props.roles, props.role?.id ?? null));
const nameValid = computed(() => isValidRoleName(name.value.trim()));

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return;
        error.value = '';
        name.value = props.role?.name ?? '';
        description.value = props.role?.description ?? '';
        parentIds.value = [...(props.role?.parent_ids ?? [])];
    },
    { immediate: true },
);

const handleSubmit = async () => {
    error.value = '';
    loading.value = true;
    // Parents that are no longer candidates (deleted meanwhile) are dropped
    const allowed = new Set(candidates.value.map((r) => r.id));
    const input = {
        name: name.value.trim(),
        description: description.value.trim(),
        parent_ids: parentIds.value.filter((id) => allowed.has(id)),
    };
    try {
        if (props.role) await updateRole(props.role.id, input);
        else await createRole(input);
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the role';
    } finally {
        loading.value = false;
    }
};

const inputClass = `border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
    focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm text-white transition-all
    focus:ring-1 focus:outline-none disabled:opacity-50`;
const labelClass = 'text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase';
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">
                    {{ editing ? `Edit role ${role?.name}` : 'Create Role' }}
                </h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>
                <form class="space-y-3" @submit.prevent="handleSubmit">
                    <div>
                        <label :class="labelClass">Name</label>
                        <input
                            v-model="name"
                            type="text"
                            :disabled="nameLocked"
                            :class="inputClass"
                            placeholder="e.g. viewer"
                        />
                        <p v-if="nameLocked" class="text-dim mt-1 text-[10px]">
                            The superuser role cannot be renamed.
                        </p>
                        <p v-else-if="name && !nameValid" class="text-lava/80 mt-1 text-[10px]">
                            2 to 50 characters: lowercase letters, digits, "-" and "_", starting
                            with a letter or digit.
                        </p>
                    </div>
                    <div>
                        <label :class="labelClass">Description</label>
                        <input v-model="description" type="text" :class="inputClass" />
                    </div>
                    <div>
                        <label :class="labelClass">Parent roles</label>
                        <SettingsUsersParentPicker v-model="parentIds" :candidates="candidates" />
                        <p class="text-dim mt-1 text-[10px]">
                            The role inherits its parents' permissions; a deny from any parent wins.
                        </p>
                    </div>
                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            @click="emit('close')"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading || !name.trim()"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            {{ editing ? 'Save' : 'Create' }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
