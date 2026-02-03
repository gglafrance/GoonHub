<script setup lang="ts">
const props = defineProps<{
    visible: boolean;
    folderName: string;
    storagePathId: number;
    folderPath: string;
}>();

const emit = defineEmits<{
    close: [];
    deleted: [];
}>();

const { getFolderSceneIDs, bulkDeleteScenes } = useApiExplorer();

const loading = ref(false);
const loadingSceneIds = ref(false);
const error = ref('');
const permanent = ref(false);
const sceneIds = ref<number[]>([]);

// Fetch scene IDs when modal opens
watch(
    () => props.visible,
    async (isVisible) => {
        if (isVisible) {
            error.value = '';
            permanent.value = false;
            sceneIds.value = [];
            loadingSceneIds.value = true;
            try {
                const response = await getFolderSceneIDs({
                    storage_path_id: props.storagePathId,
                    folder_path: props.folderPath,
                    recursive: true,
                });
                sceneIds.value = response.scene_ids;
            } catch (e: unknown) {
                error.value = e instanceof Error ? e.message : 'Failed to fetch folder contents';
            } finally {
                loadingSceneIds.value = false;
            }
        }
    },
);

const handleDelete = async () => {
    if (sceneIds.value.length === 0) {
        emit('deleted');
        return;
    }

    error.value = '';
    loading.value = true;
    try {
        await bulkDeleteScenes({
            scene_ids: sceneIds.value,
            permanent: permanent.value,
        });
        emit('deleted');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete folder contents';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    permanent.value = false;
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
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <h3 class="mb-2 text-sm font-semibold text-white">Delete Folder</h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <p class="text-dim mb-4 text-xs">
                    Are you sure you want to delete
                    <span class="text-white">"{{ folderName }}"</span>
                    and all its contents?
                </p>

                <!-- Scene count -->
                <div class="bg-surface/50 border-border mb-4 rounded-lg border p-3">
                    <div v-if="loadingSceneIds" class="text-dim flex items-center gap-2 text-xs">
                        <Icon name="heroicons:arrow-path" size="14" class="animate-spin" />
                        Counting scenes...
                    </div>
                    <div v-else class="text-xs">
                        <span class="font-medium text-white">{{ sceneIds.length }}</span>
                        <span class="text-dim">
                            {{ sceneIds.length === 1 ? ' scene' : ' scenes' }} will be
                            {{ permanent ? 'permanently deleted' : 'moved to trash' }}
                        </span>
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
                            <span class="text-xs font-medium text-white">Permanently delete</span>
                            <p class="text-dim mt-0.5 text-xs">
                                Skip trash and delete immediately. Video files will be removed from
                                disk. This cannot be undone.
                            </p>
                        </div>
                    </label>
                </div>

                <p v-if="!permanent" class="text-dim mb-4 text-xs">
                    Scenes will be moved to trash and automatically deleted after the retention
                    period. Video files will be preserved until permanent deletion.
                </p>

                <div class="flex justify-end gap-2">
                    <button
                        @click="handleClose"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white"
                    >
                        Cancel
                    </button>
                    <button
                        @click="handleDelete"
                        :disabled="loading || loadingSceneIds"
                        class="rounded-lg px-4 py-1.5 text-xs font-semibold text-white
                            transition-all disabled:cursor-not-allowed disabled:opacity-40"
                        :class="
                            permanent
                                ? 'bg-red-600 hover:bg-red-500'
                                : 'bg-amber-600 hover:bg-amber-500'
                        "
                    >
                        <template v-if="loading">
                            <Icon name="heroicons:arrow-path" size="14" class="mr-1 animate-spin" />
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
