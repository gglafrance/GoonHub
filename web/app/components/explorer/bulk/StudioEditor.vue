<script setup lang="ts">
defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const { fetchFilterOptions } = useApiVideos();
const { bulkUpdateStudio } = useApiExplorer();

const studios = ref<string[]>([]);
const studio = ref('');
const loading = ref(false);
const loadingStudios = ref(false);
const error = ref<string | null>(null);

onMounted(async () => {
    loadingStudios.value = true;
    try {
        const res = await fetchFilterOptions();
        studios.value = res.studios || [];
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load studios';
    } finally {
        loadingStudios.value = false;
    }
});

const filteredStudios = computed(() => {
    if (!studio.value) return studios.value;
    const query = studio.value.toLowerCase();
    return studios.value.filter((s) => s.toLowerCase().includes(query));
});

const selectStudio = (s: string) => {
    studio.value = s;
};

const handleSubmit = async () => {
    loading.value = true;
    error.value = null;

    try {
        await bulkUpdateStudio({
            video_ids: explorerStore.getSelectedVideoIDs(),
            studio: studio.value,
        });
        emit('complete');
        emit('close');
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to update studio';
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
            <div
                class="border-border bg-panel w-full max-w-md rounded-xl border shadow-2xl"
            >
                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 py-3">
                    <h2 class="text-sm font-semibold text-white">Bulk Edit Studio</h2>
                    <button
                        @click="$emit('close')"
                        class="text-dim hover:text-white transition-colors"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Content -->
                <div class="p-4">
                    <p class="text-dim mb-4 text-xs">
                        Set studio for {{ explorerStore.selectionCount }} videos
                    </p>

                    <!-- Error -->
                    <ErrorAlert v-if="error" :message="error" class="mb-4" />

                    <!-- Studio Input -->
                    <div class="mb-4">
                        <label class="text-dim mb-2 block text-[11px] font-medium uppercase tracking-wider">
                            Studio Name
                        </label>
                        <input
                            v-model="studio"
                            type="text"
                            placeholder="Enter or select a studio..."
                            class="border-border bg-surface focus:border-lava/50 focus:ring-lava/20
                                w-full rounded-lg border py-2 px-3 text-xs text-white
                                placeholder-white/40 transition-all focus:ring-2 focus:outline-none"
                        />
                        <p class="text-dim mt-1.5 text-[10px]">
                            Leave empty to clear studio from selected videos
                        </p>
                    </div>

                    <!-- Existing Studios -->
                    <div v-if="studios.length > 0" class="mb-4">
                        <label class="text-dim mb-2 block text-[11px] font-medium uppercase tracking-wider">
                            Existing Studios
                        </label>

                        <div v-if="loadingStudios" class="flex items-center justify-center py-4">
                            <LoadingSpinner />
                        </div>

                        <div v-else class="max-h-48 overflow-y-auto">
                            <div class="flex flex-wrap gap-1.5">
                                <button
                                    v-for="s in filteredStudios"
                                    :key="s"
                                    @click="selectStudio(s)"
                                    class="rounded-lg border px-2.5 py-1 text-[11px] font-medium
                                        transition-all"
                                    :class="
                                        studio === s
                                            ? 'border-lava bg-lava/10 text-lava'
                                            : 'border-border hover:border-border-hover text-dim hover:text-white'
                                    "
                                >
                                    {{ s }}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Footer -->
                <div class="border-border flex items-center justify-end gap-2 border-t px-4 py-3">
                    <button
                        @click="$emit('close')"
                        class="border-border hover:border-border-hover rounded-lg border px-3 py-1.5
                            text-xs font-medium text-white transition-all"
                    >
                        Cancel
                    </button>
                    <button
                        @click="handleSubmit"
                        :disabled="loading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs font-semibold
                            text-white transition-colors disabled:opacity-50"
                    >
                        <span v-if="loading">Applying...</span>
                        <span v-else>Apply</span>
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
