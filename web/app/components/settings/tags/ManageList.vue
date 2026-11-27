<script setup lang="ts">
import type { Tag } from '~/types/tag';

// List of all tags: search, rename, color change, delete, and a selection to merge or delete
// several tags.
const props = defineProps<{
    tags: Tag[];
    loading: boolean;
}>();

const emit = defineEmits<{
    delete: [tag: Tag];
    color: [tag: Tag, color: string];
    merge: [ids: number[]];
    deleteMany: [ids: number[]];
    renamed: [tag: Tag];
    // rename refused because target already has the name: merge source into it instead
    mergeInto: [source: Tag, target: Tag];
}>();

const query = ref('');
const selected = ref<number[]>([]);

const visibleTags = computed(() => filterTagsByName(props.tags, query.value));
const allVisibleSelected = computed(
    () =>
        visibleTags.value.length > 0 &&
        visibleTags.value.every((t) => selected.value.includes(t.id)),
);

// drop selected tags that no longer exist (merged or deleted)
watch(
    () => props.tags,
    (tags) => {
        selected.value = selected.value.filter((id) => tags.some((t) => t.id === id));
    },
);

function toggleAllVisible() {
    const ids = visibleTags.value.map((t) => t.id);
    selected.value = allVisibleSelected.value
        ? selected.value.filter((id) => !ids.includes(id))
        : [...new Set([...selected.value, ...ids])];
}

function clearSelection() {
    selected.value = [];
}

defineExpose({ clearSelection });
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4 flex flex-wrap items-center gap-3">
            <h3 class="text-sm font-semibold text-white">Manage Tags</h3>
            <span v-if="tags.length > 0" class="text-dim text-[11px]">{{
                plural(tags.length, 'tag')
            }}</span>
            <input
                v-if="tags.length > 0"
                v-model="query"
                type="text"
                placeholder="Filter tags..."
                class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 ml-auto w-48
                    rounded-lg border px-3 py-1.5 text-xs text-white transition-all focus:ring-1
                    focus:outline-none"
            />
        </div>

        <div
            v-if="selected.length > 0"
            class="border-lava/20 bg-lava/5 mb-3 flex items-center gap-3 rounded-lg border px-3
                py-2"
        >
            <span class="text-xs text-white">{{ selected.length }} selected</span>
            <button
                class="text-dim text-[11px] transition-colors hover:text-white"
                @click="clearSelection"
            >
                Clear
            </button>
            <button
                class="border-border text-dim ml-auto flex items-center gap-1.5 rounded-lg border
                    px-3 py-1 text-[11px] font-semibold transition-all hover:border-red-500/40
                    hover:text-red-400"
                title="Remove the selected tags from their videos (the videos are kept)"
                @click="emit('deleteMany', [...selected])"
            >
                <Icon name="heroicons:trash" size="13" />
                Delete
            </button>
            <button
                class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-3 py-1
                    text-[11px] font-semibold text-white transition-all"
                @click="emit('merge', [...selected])"
            >
                <Icon name="heroicons:arrows-pointing-in" size="13" />
                Merge into...
            </button>
        </div>

        <div v-if="loading" class="flex items-center gap-2 py-4">
            <LoadingSpinner />
        </div>

        <div v-else-if="tags.length === 0" class="text-dim py-4 text-center text-xs">
            No tags created yet.
        </div>

        <div v-else-if="visibleTags.length === 0" class="text-dim py-4 text-center text-xs">
            No tag matches "{{ query }}".
        </div>

        <div v-else class="space-y-1">
            <label class="text-dim flex cursor-pointer items-center gap-3 px-3 py-1 text-[11px]">
                <input
                    type="checkbox"
                    :checked="allVisibleSelected"
                    class="accent-lava h-3.5 w-3.5 rounded"
                    @change="toggleAllVisible"
                />
                Select {{ query ? 'matching' : 'all' }}
            </label>
            <div
                v-for="tag in visibleTags"
                :key="tag.id"
                class="border-border group flex items-center gap-3 rounded-lg border px-3 py-2
                    transition-colors hover:bg-white/2"
                :class="selected.includes(tag.id) ? 'border-lava/30 bg-lava/3' : ''"
            >
                <input
                    v-model="selected"
                    type="checkbox"
                    :value="tag.id"
                    class="accent-lava h-3.5 w-3.5 rounded"
                    :aria-label="`Select ${tag.name}`"
                />
                <SettingsTagsColorPicker
                    :color="tag.color"
                    @select="(c) => emit('color', tag, c)"
                />
                <SettingsTagsNameEditor
                    :tag="tag"
                    @renamed="(t) => emit('renamed', t)"
                    @merge-into="(target) => emit('mergeInto', tag, target)"
                />
                <span class="text-dim text-[11px]">{{ plural(tag.scene_count, 'scene') }}</span>
                <button
                    class="text-dim flex items-center justify-center opacity-0 transition-all
                        group-hover:opacity-100 hover:text-red-400"
                    title="Delete tag"
                    @click="emit('delete', tag)"
                >
                    <Icon name="heroicons:trash" size="14" />
                </button>
            </div>
        </div>
    </div>
</template>
