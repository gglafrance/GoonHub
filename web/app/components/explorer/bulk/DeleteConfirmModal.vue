<script setup lang="ts">
defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const { bulkDeleteScenes } = useApiExplorer();

const loading = ref(false);
const error = ref<string | null>(null);
const permanent = ref(false);

const handleConfirm = async () => {
    loading.value = true;
    error.value = null;

    try {
        await bulkDeleteScenes({
            scene_ids: explorerStore.getSelectedSceneIDs(),
            permanent: permanent.value,
        });
        emit('complete');
        emit('close');
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to delete scenes';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    permanent.value = false;
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="border-border bg-panel w-full max-w-sm rounded-xl border shadow-2xl">
                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 py-3">
                    <h2 class="text-sm font-semibold text-white">Delete Scenes</h2>
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="handleClose"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Content -->
                <div class="p-4">
                    <div class="mb-4 flex items-start gap-3">
                        <div
                            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
                            :class="permanent ? 'bg-red-500/10' : 'bg-amber-500/10'"
                        >
                            <Icon
                                name="heroicons:exclamation-triangle"
                                size="20"
                                :class="permanent ? 'text-red-400' : 'text-amber-400'"
                            />
                        </div>
                        <div>
                            <p class="text-sm font-medium text-white">
                                {{ permanent ? 'Delete' : 'Move to trash' }}
                                {{ explorerStore.selectionCount }} scene{{
                                    explorerStore.selectionCount === 1 ? '' : 's'
                                }}?
                            </p>
                            <p class="text-dim mt-1 text-xs">
                                <template v-if="permanent">
                                    This will permanently delete the scene files, thumbnails, and
                                    all associated data. This action cannot be undone.
                                </template>
                                <template v-else>
                                    Scenes will be moved to trash and automatically deleted after
                                    the retention period. Video files will be preserved until
                                    permanent deletion.
                                </template>
                            </p>
                        </div>
                    </div>

                    <!-- Permanent delete checkbox -->
                    <div class="bg-surface/50 border-border mb-4 rounded-lg border p-3">
                        <label class="flex cursor-pointer items-start gap-3">
                            <input
                                v-model="permanent"
                                type="checkbox"
                                class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                            />
                            <div>
                                <span class="text-xs font-medium text-white"
                                    >Permanently delete</span
                                >
                                <p class="text-dim mt-0.5 text-xs">
                                    Skip trash and delete immediately. Video files will be removed
                                    from disk. This cannot be undone.
                                </p>
                            </div>
                        </label>
                    </div>

                    <!-- Error -->
                    <ErrorAlert v-if="error" :message="error" class="mb-4" />
                </div>

                <!-- Footer -->
                <div class="border-border flex items-center justify-end gap-2 border-t px-4 py-3">
                    <button
                        :disabled="loading"
                        class="border-border hover:border-border-hover rounded-lg border px-3 py-1.5
                            text-xs font-medium text-white transition-all disabled:opacity-50"
                        @click="handleClose"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading"
                        class="rounded-lg px-3 py-1.5 text-xs font-semibold text-white
                            transition-colors disabled:opacity-50"
                        :class="
                            permanent
                                ? 'bg-red-600 hover:bg-red-500'
                                : 'bg-amber-600 hover:bg-amber-500'
                        "
                        @click="handleConfirm"
                    >
                        <template v-if="loading">
                            <Icon
                                name="heroicons:arrow-path"
                                size="14"
                                class="mr-1 inline animate-spin"
                            />
                            {{ permanent ? 'Deleting...' : 'Moving to trash...' }}
                        </template>
                        <template v-else>
                            {{ permanent ? 'Delete Permanently' : 'Move to Trash' }}
                        </template>
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
