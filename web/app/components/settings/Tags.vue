<script setup lang="ts">
import type { Tag } from '~/types/tag';
import type { TagSort } from '~/types/settings';

const { fetchTags, createTag, deleteTag } = useApi();
const { message, error, clearMessages } = useSettingsMessage();
const settingsStore = useSettingsStore();

const tags = ref<Tag[]>([]);
const loading = ref(false);

const newTagName = ref('');
const newTagColor = ref('#6B7280');
const creating = ref(false);

const selectedTagSort = ref<TagSort>(settingsStore.defaultTagSort);

const tagSortOptions: { value: TagSort; label: string }[] = [
    { value: 'az', label: 'A-Z' },
    { value: 'za', label: 'Z-A' },
    { value: 'most', label: 'Most used' },
    { value: 'least', label: 'Least used' },
];

watch(
    () => settingsStore.defaultTagSort,
    (val) => {
        selectedTagSort.value = val;
    },
);

async function handleSortChange() {
    clearMessages();
    try {
        await settingsStore.updateTags(selectedTagSort.value);
        message.value = 'Default tag sort updated';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update tag sort';
    }
}

const colorPresets = [
    '#6B7280',
    '#FF4D4D',
    '#F59E0B',
    '#8B5CF6',
    '#3B82F6',
    '#EC4899',
    '#F97316',
    '#14B8A6',
    '#22C55E',
    '#06B6D4',
    '#A78BFA',
    '#6366F1',
    '#EF4444',
];

onMounted(async () => {
    await loadTags();
});

async function loadTags() {
    loading.value = true;
    try {
        const res = await fetchTags();
        tags.value = res.data || [];
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load tags';
    } finally {
        loading.value = false;
    }
}

async function handleCreate() {
    if (!newTagName.value.trim()) return;
    clearMessages();
    creating.value = true;

    try {
        const tag = await createTag(newTagName.value.trim(), newTagColor.value);
        tags.value.push(tag);
        tags.value.sort((a, b) => a.name.localeCompare(b.name));
        message.value = `Tag "${tag.name}" created`;
        newTagName.value = '';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to create tag';
    } finally {
        creating.value = false;
    }
}

async function handleDelete(tag: Tag) {
    clearMessages();
    try {
        await deleteTag(tag.id);
        tags.value = tags.value.filter((t) => t.id !== tag.id);
        message.value = `Tag "${tag.name}" deleted`;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete tag';
    }
}
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <!-- Default sort -->
        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">Preferences</h3>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Default Sort
                </label>
                <UiSelectMenu
                    :model-value="selectedTagSort"
                    @update:model-value="
                        selectedTagSort = $event as TagSort;
                        handleSortChange();
                    "
                    :options="tagSortOptions"
                    class="max-w-48"
                />
                <p class="text-dim mt-1.5 text-[10px]">
                    Default sort order when opening the tag picker
                </p>
            </div>
        </div>

        <!-- Create tag -->
        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">Create Tag</h3>
            <div class="space-y-4">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Name
                    </label>
                    <input
                        v-model="newTagName"
                        type="text"
                        placeholder="Tag name"
                        maxlength="100"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full max-w-64 rounded-lg border px-3.5 py-2.5 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                        @keydown.enter="handleCreate"
                    />
                </div>
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Color
                    </label>
                    <div class="flex items-center gap-2">
                        <button
                            v-for="color in colorPresets"
                            :key="color"
                            @click="newTagColor = color"
                            class="h-6 w-6 rounded-full border-2 transition-all"
                            :class="
                                newTagColor === color
                                    ? 'scale-110 border-white'
                                    : 'border-transparent hover:scale-110'
                            "
                            :style="{ backgroundColor: color }"
                        />
                    </div>
                </div>
                <button
                    @click="handleCreate"
                    :disabled="!newTagName.trim() || creating"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                        text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                >
                    Create Tag
                </button>
            </div>
        </div>

        <!-- Existing tags -->
        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">Manage Tags</h3>

            <div v-if="loading" class="flex items-center gap-2 py-4">
                <LoadingSpinner />
            </div>

            <div v-else-if="tags.length === 0" class="text-dim py-4 text-center text-xs">
                No tags created yet.
            </div>

            <div v-else class="space-y-1">
                <div
                    v-for="tag in tags"
                    :key="tag.id"
                    class="border-border group flex items-center gap-3 rounded-lg border px-3 py-2
                        transition-colors hover:bg-white/2"
                >
                    <span class="h-3 w-3 rounded-full" :style="{ backgroundColor: tag.color }" />
                    <span class="flex-1 text-xs text-white">{{ tag.name }}</span>
                    <button
                        @click="handleDelete(tag)"
                        class="text-dim opacity-0 transition-all group-hover:opacity-100
                            hover:text-red-400"
                        title="Delete tag"
                    >
                        <Icon name="heroicons:trash" size="14" />
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
