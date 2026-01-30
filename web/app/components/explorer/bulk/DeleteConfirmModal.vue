<script setup lang="ts">
defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const { bulkDeleteVideos } = useApiExplorer();

const loading = ref(false);
const error = ref<string | null>(null);

const handleConfirm = async () => {
    loading.value = true;
    error.value = null;

    try {
        const result = await bulkDeleteVideos({
            video_ids: explorerStore.getSelectedVideoIDs(),
        });
        emit('complete');
        emit('close');
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to delete videos';
    } finally {
        loading.value = false;
    }
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="$emit('close')"
        >
            <div class="border-border bg-panel w-full max-w-sm rounded-xl border shadow-2xl">
                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 py-3">
                    <h2 class="text-sm font-semibold text-white">Delete Videos</h2>
                    <button
                        @click="$emit('close')"
                        class="text-dim hover:text-white transition-colors"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Content -->
                <div class="p-4">
                    <div class="mb-4 flex items-start gap-3">
                        <div
                            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg
                                bg-red-500/10"
                        >
                            <Icon name="heroicons:exclamation-triangle" size="20" class="text-red-400" />
                        </div>
                        <div>
                            <p class="text-sm font-medium text-white">
                                Delete {{ explorerStore.selectionCount }} video{{
                                    explorerStore.selectionCount === 1 ? '' : 's'
                                }}?
                            </p>
                            <p class="text-dim mt-1 text-xs">
                                This will permanently delete the video files, thumbnails, and all
                                associated data. This action cannot be undone.
                            </p>
                        </div>
                    </div>

                    <!-- Error -->
                    <ErrorAlert v-if="error" :message="error" class="mb-4" />
                </div>

                <!-- Footer -->
                <div class="border-border flex items-center justify-end gap-2 border-t px-4 py-3">
                    <button
                        @click="$emit('close')"
                        :disabled="loading"
                        class="border-border hover:border-border-hover rounded-lg border px-3 py-1.5
                            text-xs font-medium text-white transition-all disabled:opacity-50"
                    >
                        Cancel
                    </button>
                    <button
                        @click="handleConfirm"
                        :disabled="loading"
                        class="rounded-lg bg-red-500 px-3 py-1.5 text-xs font-semibold text-white
                            transition-colors hover:bg-red-600 disabled:opacity-50"
                    >
                        <span v-if="loading">Deleting...</span>
                        <span v-else>Delete</span>
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
