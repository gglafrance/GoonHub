<script setup lang="ts">
import type { Actor } from '~/types/actor';

const props = defineProps<{
    visible: boolean;
    actor: Actor;
}>();

const emit = defineEmits<{
    close: [];
    deleted: [];
}>();

const { fetchAllActorSceneIDs, deleteActor } = useApiActors();
const { bulkDeleteScenes } = useApiExplorer();

const loading = ref(false);
const loadingSceneIds = ref(false);
const error = ref('');
const deleteScenes = ref(false);
const permanent = ref(false);
const sceneIds = ref<number[]>([]);

// Fetch scene IDs when modal opens
watch(
    () => props.visible,
    async (isVisible) => {
        if (isVisible) {
            error.value = '';
            deleteScenes.value = false;
            permanent.value = false;
            sceneIds.value = [];
            loadingSceneIds.value = true;
            try {
                sceneIds.value = await fetchAllActorSceneIDs(props.actor.uuid, props.actor.name);
            } catch (e: unknown) {
                error.value = e instanceof Error ? e.message : 'Failed to fetch actor scenes';
            } finally {
                loadingSceneIds.value = false;
            }
        }
    },
    { immediate: true },
);

const handleDelete = async () => {
    error.value = '';
    loading.value = true;
    try {
        // Delete scenes first if requested
        if (deleteScenes.value && sceneIds.value.length > 0) {
            await bulkDeleteScenes({
                scene_ids: sceneIds.value,
                permanent: permanent.value,
            });
        }
        // Then delete the actor
        await deleteActor(props.actor.id);
        emit('deleted');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete actor';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    deleteScenes.value = false;
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
                <h3 class="mb-2 text-sm font-semibold text-white">Delete Actor</h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <p class="text-dim mb-4 text-xs">
                    Are you sure you want to delete
                    <span class="text-white">"{{ actor.name }}"</span>?
                </p>

                <!-- Scene count -->
                <div class="bg-surface/50 border-border mb-4 rounded-lg border p-3">
                    <div v-if="loadingSceneIds" class="text-dim flex items-center gap-2 text-xs">
                        <Icon name="heroicons:arrow-path" size="14" class="animate-spin" />
                        Counting scenes...
                    </div>
                    <div v-else class="text-xs">
                        <span class="text-dim">This actor has </span>
                        <span class="font-medium text-white">{{ sceneIds.length }}</span>
                        <span class="text-dim">
                            {{ sceneIds.length === 1 ? ' scene' : ' scenes' }}
                        </span>
                    </div>
                </div>

                <!-- Delete scenes checkbox -->
                <div
                    v-if="sceneIds.length > 0"
                    class="bg-surface/50 border-border mb-4 rounded-lg border p-3"
                >
                    <label class="flex cursor-pointer items-start gap-3">
                        <input
                            v-model="deleteScenes"
                            type="checkbox"
                            class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                        />
                        <div>
                            <span class="text-xs font-medium text-white"
                                >Delete associated scenes</span
                            >
                            <p class="text-dim mt-0.5 text-xs">
                                Also delete all {{ sceneIds.length }}
                                {{ sceneIds.length === 1 ? 'scene' : 'scenes' }} associated with
                                this actor.
                            </p>
                        </div>
                    </label>

                    <!-- Permanent delete sub-checkbox -->
                    <div v-if="deleteScenes" class="mt-3 ml-7">
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
                </div>

                <p v-if="deleteScenes && !permanent" class="text-dim mb-4 text-xs">
                    Scenes will be moved to trash and automatically deleted after the retention
                    period. Video files will be preserved until permanent deletion.
                </p>

                <p v-if="!deleteScenes" class="text-dim mb-4 text-xs">
                    The actor will be removed from the system. Associated scenes will remain but
                    will no longer be linked to this actor.
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
                        :disabled="loading || loadingSceneIds"
                        class="rounded-lg px-4 py-1.5 text-xs font-semibold text-white
                            transition-all disabled:cursor-not-allowed disabled:opacity-40"
                        :class="
                            deleteScenes && permanent
                                ? 'bg-red-600 hover:bg-red-500'
                                : 'bg-amber-600 hover:bg-amber-500'
                        "
                        @click="handleDelete"
                    >
                        <template v-if="loading">
                            <Icon name="heroicons:arrow-path" size="14" class="mr-1 animate-spin" />
                            {{ deleteScenes && permanent ? 'Deleting...' : 'Deleting...' }}
                        </template>
                        <template v-else>
                            {{
                                deleteScenes && permanent
                                    ? 'Delete Permanently'
                                    : deleteScenes
                                      ? 'Delete & Trash Scenes'
                                      : 'Delete Actor'
                            }}
                        </template>
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
