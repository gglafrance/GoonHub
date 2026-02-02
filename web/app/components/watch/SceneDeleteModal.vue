<script setup lang="ts">
const props = defineProps<{
    visible: boolean;
    sceneId: number;
    sceneTitle: string;
}>();

const emit = defineEmits<{
    close: [];
    deleted: [];
}>();

const { deleteScene } = useApiScenes();

const loading = ref(false);
const error = ref('');
const permanent = ref(false);

const handleDelete = async () => {
    error.value = '';
    loading.value = true;
    try {
        await deleteScene(props.sceneId, permanent.value);
        emit('deleted');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete scene';
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
                <h3 class="mb-2 text-sm font-semibold text-white">Delete Scene</h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2 text-xs"
                >
                    {{ error }}
                </div>
                <p class="text-dim mb-4 text-xs">
                    Are you sure you want to delete
                    <span class="text-white">"{{ sceneTitle }}"</span>?
                </p>

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
                                Skip trash and delete immediately. This cannot be undone.
                            </p>
                        </div>
                    </label>
                </div>

                <p v-if="!permanent" class="text-dim mb-4 text-xs">
                    The scene will be moved to trash and automatically deleted after the retention
                    period.
                </p>

                <div class="flex justify-end gap-2">
                    <button
                        @click="handleClose"
                        class="text-dim hover:text-white rounded-lg px-3 py-1.5 text-xs transition-colors"
                    >
                        Cancel
                    </button>
                    <button
                        @click="handleDelete"
                        :disabled="loading"
                        class="rounded-lg px-4 py-1.5 text-xs font-semibold text-white transition-all
                            disabled:cursor-not-allowed disabled:opacity-40"
                        :class="
                            permanent
                                ? 'bg-red-600 hover:bg-red-500'
                                : 'bg-amber-600 hover:bg-amber-500'
                        "
                    >
                        {{ permanent ? 'Delete Permanently' : 'Move to Trash' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
