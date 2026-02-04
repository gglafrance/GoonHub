<script setup lang="ts">
import type { AdminUser } from '~/types/admin';

const props = defineProps<{
    visible: boolean;
    user: AdminUser | null;
}>();

const emit = defineEmits<{
    close: [];
    deleted: [];
}>();

const { deleteUser } = useApi();

const loading = ref(false);
const error = ref('');

const handleDelete = async () => {
    if (!props.user) return;
    error.value = '';
    loading.value = true;
    try {
        await deleteUser(props.user.id);
        emit('deleted');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete user';
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
                <h3 class="mb-2 text-sm font-semibold text-white">Delete User</h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>
                <p class="text-dim mb-4 text-xs">
                    Are you sure you want to delete
                    <span class="text-white">{{ user?.username }}</span
                    >? This action cannot be undone.
                </p>
                <div class="flex justify-end gap-2">
                    <button
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white"
                        @click="handleClose"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading"
                        class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold text-white
                            transition-all hover:bg-red-500 disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="handleDelete"
                    >
                        Delete
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
