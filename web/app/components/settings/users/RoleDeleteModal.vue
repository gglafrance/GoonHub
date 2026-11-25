<script setup lang="ts">
import type { RoleSummary } from '~/types/permissions';

// Delete confirmation of a role. The API refuses system roles and roles still held by users.
const props = defineProps<{
    visible: boolean;
    role: RoleSummary | null;
}>();

const emit = defineEmits<{
    close: [];
    deleted: [];
}>();

const { deleteRole } = useApiAdmin();

const loading = ref(false);
const error = ref('');

// Why the deletion will be refused, shown before trying
const blocker = computed(() => {
    const role = props.role;
    if (!role) return '';
    if (role.is_system) return 'This is a built-in role: it cannot be deleted.';
    if (role.user_count > 0) {
        const users = role.user_count === 1 ? '1 user has' : `${role.user_count} users have`;
        return `${users} this role: give them another role first.`;
    }
    return '';
});

watch(
    () => props.visible,
    () => {
        error.value = '';
    },
);

const handleDelete = async () => {
    if (!props.role) return;
    error.value = '';
    loading.value = true;
    try {
        await deleteRole(props.role.id);
        emit('deleted');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete the role';
    } finally {
        loading.value = false;
    }
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="glass-panel border-border w-full max-w-sm border p-6">
                <h3 class="mb-2 text-sm font-semibold text-white">Delete Role</h3>
                <p class="text-dim mb-3 text-xs">
                    Delete the role <span class="text-white">{{ role?.name }}</span
                    >? Roles that inherit from it lose it as a parent.
                </p>
                <div
                    v-if="blocker"
                    class="mb-3 rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2
                        text-xs text-amber-400"
                >
                    {{ blocker }}
                </div>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>
                <div class="flex justify-end gap-2">
                    <button
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white"
                        @click="emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading || !!blocker"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                            font-semibold text-white transition-all disabled:cursor-not-allowed
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
