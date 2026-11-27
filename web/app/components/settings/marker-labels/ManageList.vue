<script setup lang="ts">
import type { MarkerLabelWithUsage } from '~/types/markerLabel';
import type { Tag } from '~/types/tag';

// List of the marker labels: search, rename, recolor, description, default tags, delete, and a
// selection to merge or delete several labels.
const props = defineProps<{
    labels: MarkerLabelWithUsage[];
    loading: boolean;
}>();

const emit = defineEmits<{
    color: [label: MarkerLabelWithUsage, color: string];
    renamed: [label: MarkerLabelWithUsage, name: string];
    tags: [label: MarkerLabelWithUsage, tags: Tag[]];
    merge: [ids: number[]];
    // rename refused because target already has the name: merge source into it instead
    mergeInto: [source: MarkerLabelWithUsage, target: { id: number; name: string }];
    delete: [ids: number[]];
}>();

const query = ref('');
const selected = ref<number[]>([]);

const visible = computed(() => filterTagsByName(props.labels, query.value));
const allVisibleSelected = computed(
    () => visible.value.length > 0 && visible.value.every((l) => selected.value.includes(l.id)),
);

// drop selected labels that no longer exist (merged or deleted)
watch(
    () => props.labels,
    (labels) => {
        selected.value = selected.value.filter((id) => labels.some((l) => l.id === id));
    },
);

function toggleAllVisible() {
    const ids = visible.value.map((l) => l.id);
    selected.value = allVisibleSelected.value
        ? selected.value.filter((id) => !ids.includes(id))
        : [...new Set([...selected.value, ...ids])];
}

function toggle(id: number, on: boolean) {
    selected.value = on
        ? [...new Set([...selected.value, id])]
        : selected.value.filter((s) => s !== id);
}

function clearSelection() {
    selected.value = [];
}

defineExpose({ clearSelection });
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4 flex flex-wrap items-center gap-3">
            <h3 class="text-sm font-semibold text-white">Manage Labels</h3>
            <span v-if="labels.length > 0" class="text-dim text-[11px]">{{
                plural(labels.length, 'label')
            }}</span>
            <input
                v-if="labels.length > 0"
                v-model="query"
                type="text"
                placeholder="Filter labels..."
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
                title="Delete the selected labels (their markers are kept, without label)"
                @click="emit('delete', [...selected])"
            >
                <Icon name="heroicons:trash" size="13" />
                Delete
            </button>
            <button
                v-if="selected.length > 1"
                class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-3 py-1
                    text-[11px] font-semibold text-white transition-all"
                @click="emit('merge', [...selected])"
            >
                <Icon name="heroicons:arrows-pointing-in" size="13" />
                Merge...
            </button>
        </div>

        <div v-if="loading" class="flex items-center gap-2 py-4">
            <LoadingSpinner />
        </div>

        <div v-else-if="labels.length === 0" class="text-dim py-4 text-center text-xs">
            No marker labels yet.
        </div>

        <div v-else-if="visible.length === 0" class="text-dim py-4 text-center text-xs">
            No label matches "{{ query }}".
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
            <SettingsMarkerLabelsRow
                v-for="label in visible"
                :key="label.id"
                :label="label"
                :selected="selected.includes(label.id)"
                @select="(on) => toggle(label.id, on)"
                @color="(c) => emit('color', label, c)"
                @renamed="(name) => emit('renamed', label, name)"
                @tags="(tags) => emit('tags', label, tags)"
                @merge-into="(target) => emit('mergeInto', label, target)"
                @delete="emit('delete', [label.id])"
            />
        </div>
    </div>
</template>
