<script setup lang="ts">
import type { AdminUser, RoleResponse } from '~/types/admin';

const props = defineProps<{
    visible: boolean;
    user: AdminUser | null;
    roles: RoleResponse[];
}>();

const emit = defineEmits<{
    close: [];
    updated: [];
}>();

const { updateUserRole } = useApi();

const roleValue = ref('');
const loading = ref(false);
const error = ref('');

watch(
    () => props.user,
    (u) => {
        if (u) {
            roleValue.value = u.role;
        }
    },
);

const handleSubmit = async () => {
    if (!props.user) return;
    error.value = '';
    loading.value = true;
    try {
        await updateUserRole(props.user.id, roleValue.value);
        emit('updated');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update role';
    } finally {
        loading.value = false;
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
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border w-full max-w-sm border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">
                    Change Role for {{ user?.username }}
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
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Role
                        </label>
                        <UiSelectMenu
                            v-model="roleValue"
                            :options="roles.map((r) => ({ value: r.name, label: r.name }))"
                        />
                    </div>
                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            @click="handleClose"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            Update
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
