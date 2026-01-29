<script setup lang="ts">
import type { Tag } from '~/types/tag';

defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const { fetchTags } = useApiTags();
const { bulkUpdateTags } = useApiExplorer();

const allTags = ref<Tag[]>([]);
const selectedTagIDs = ref<Set<number>>(new Set());
const mode = ref<'add' | 'remove' | 'replace'>('add');
const loading = ref(false);
const loadingTags = ref(false);
const error = ref<string | null>(null);

onMounted(async () => {
    loadingTags.value = true;
    try {
        const res = await fetchTags();
        allTags.value = res.data || [];
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load tags';
    } finally {
        loadingTags.value = false;
    }
});

const toggleTag = (tagId: number) => {
    if (selectedTagIDs.value.has(tagId)) {
        selectedTagIDs.value.delete(tagId);
    } else {
        selectedTagIDs.value.add(tagId);
    }
    selectedTagIDs.value = new Set(selectedTagIDs.value);
};

const isTagSelected = (tagId: number) => selectedTagIDs.value.has(tagId);

const handleSubmit = async () => {
    if (selectedTagIDs.value.size === 0 && mode.value !== 'replace') {
        error.value = 'Select at least one tag';
        return;
    }

    loading.value = true;
    error.value = null;

    try {
        await bulkUpdateTags({
            video_ids: explorerStore.getSelectedVideoIDs(),
            tag_ids: Array.from(selectedTagIDs.value),
            mode: mode.value,
        });
        emit('complete');
        emit('close');
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
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
                    <h2 class="text-sm font-semibold text-white">Bulk Edit Tags</h2>
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
                        Editing tags for {{ explorerStore.selectionCount }} videos
                    </p>

                    <!-- Mode Selection -->
                    <div class="mb-4">
                        <label class="text-dim mb-2 block text-[11px] font-medium uppercase tracking-wider">
                            Mode
                        </label>
                        <div class="flex gap-2">
                            <button
                                v-for="m in ['add', 'remove', 'replace'] as const"
                                :key="m"
                                @click="mode = m"
                                class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-all"
                                :class="
                                    mode === m
                                        ? 'border-lava bg-lava/10 text-lava'
                                        : 'border-border hover:border-border-hover text-dim hover:text-white'
                                "
                            >
                                {{ m.charAt(0).toUpperCase() + m.slice(1) }}
                            </button>
                        </div>
                        <p class="text-dim mt-1.5 text-[10px]">
                            <template v-if="mode === 'add'">Add selected tags to existing tags</template>
                            <template v-else-if="mode === 'remove'">Remove selected tags from videos</template>
                            <template v-else>Replace all tags with selected tags</template>
                        </p>
                    </div>

                    <!-- Error -->
                    <ErrorAlert v-if="error" :message="error" class="mb-4" />

                    <!-- Tag Selection -->
                    <div class="mb-4">
                        <label class="text-dim mb-2 block text-[11px] font-medium uppercase tracking-wider">
                            Tags
                        </label>

                        <div v-if="loadingTags" class="flex items-center justify-center py-4">
                            <LoadingSpinner />
                        </div>

                        <div v-else-if="allTags.length === 0" class="text-dim py-4 text-center text-xs">
                            No tags available
                        </div>

                        <div v-else class="max-h-64 overflow-y-auto">
                            <div class="flex flex-wrap gap-1.5">
                                <button
                                    v-for="tag in allTags"
                                    :key="tag.id"
                                    @click="toggleTag(tag.id)"
                                    class="flex items-center gap-1.5 rounded-full border px-2.5 py-1
                                        text-[11px] font-medium transition-all"
                                    :class="
                                        isTagSelected(tag.id)
                                            ? 'ring-2'
                                            : 'opacity-70 hover:opacity-100'
                                    "
                                    :style="{
                                        borderColor: tag.color + '60',
                                        backgroundColor: tag.color + '15',
                                        color: 'white',
                                        '--tw-ring-color': tag.color,
                                    }"
                                >
                                    <span
                                        class="inline-block h-2 w-2 rounded-full"
                                        :style="{ backgroundColor: tag.color }"
                                    />
                                    {{ tag.name }}
                                    <Icon
                                        v-if="isTagSelected(tag.id)"
                                        name="heroicons:check"
                                        size="10"
                                    />
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
