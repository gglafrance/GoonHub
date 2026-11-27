<script setup lang="ts">
defineProps<{
    visible: boolean;
    count: number;
    size: string;
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
                <h3 class="mb-2 text-sm font-semibold text-white">Clear Transcode Cache</h3>
                <p class="text-dim mb-4 text-xs">
                    Remove
                    <span class="text-white"
                        >{{ count }} cached conversion{{ count === 1 ? '' : 's' }} ({{
                            size
                        }})</span
                    >? Scenes are converted again the next time they are played. Conversions being
                    watched are removed when their viewers leave. Original video files are not
                    touched.
                </p>
                <div class="flex justify-end gap-2">
                    <button
                        :disabled="loading"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white disabled:opacity-40"
                        @click="emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading"
                        class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold text-white
                            transition-all hover:bg-red-500 disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="emit('confirm')"
                    >
                        {{ loading ? 'Clearing...' : 'Clear Cache' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
