<script setup lang="ts">
defineProps<{
    visible: boolean;
    count: number;
    loading: boolean;
}>();

const emit = defineEmits<{
    close: [];
    confirm: [];
}>();
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="glass-panel border-border w-full max-w-sm border p-6">
                <h3 class="mb-2 text-sm font-semibold text-white">Empty Trash</h3>
                <p class="text-dim mb-4 text-xs">
                    Are you sure you want to permanently delete
                    <span class="text-white">{{ count }} scene{{ count === 1 ? '' : 's' }}</span
                    >? This action cannot be undone.
                </p>
                <div class="flex justify-end gap-2">
                    <button
                        @click="emit('close')"
                        :disabled="loading"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white disabled:opacity-40"
                    >
                        Cancel
                    </button>
                    <button
                        @click="emit('confirm')"
                        :disabled="loading"
                        class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold text-white
                            transition-all hover:bg-red-500 disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        {{ loading ? 'Deleting...' : 'Empty Trash' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
