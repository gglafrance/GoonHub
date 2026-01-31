<script setup lang="ts">
import type { MarkerTagInfo } from '~/types/marker';
import type { Tag } from '~/types/tag';

const props = defineProps<{
    markerId: number;
}>();

const { fetchMarkerTags, setMarkerTags } = useApiMarkers();
const { fetchTags } = useApiTags();

const loading = ref(false);
const saving = ref(false);
const error = ref<string | null>(null);
const markerTags = ref<MarkerTagInfo[]>([]);
const allTags = ref<Tag[]>([]);
const allTagsLoaded = ref(false);
const loadingAllTags = ref(false);
const showTagPicker = ref(false);
const anchorRef = ref<HTMLElement | null>(null);

// Tags that can be added (not already on the marker)
const availableTags = computed(() =>
    allTags.value.filter((t) => !markerTags.value.some((mt) => mt.id === t.id)),
);

// Individual (non-label) tags that can be removed
const individualTags = computed(() => markerTags.value.filter((t) => !t.is_from_label));

// Label-derived tags (cannot be removed directly)
const labelTags = computed(() => markerTags.value.filter((t) => t.is_from_label));

onMounted(() => {
    loadMarkerTags();
});

async function loadMarkerTags() {
    loading.value = true;
    error.value = null;

    try {
        const tags = await fetchMarkerTags(props.markerId);
        markerTags.value = tags || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load tags';
        markerTags.value = [];
    } finally {
        loading.value = false;
    }
}

async function loadAllTags() {
    if (allTagsLoaded.value || loadingAllTags.value) return;
    loadingAllTags.value = true;

    try {
        const res = await fetchTags();
        allTags.value = res.data || [];
        allTagsLoaded.value = true;
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load tags';
    } finally {
        loadingAllTags.value = false;
    }
}

async function onAddTagClick() {
    if (showTagPicker.value) {
        showTagPicker.value = false;
        return;
    }
    await loadAllTags();
    showTagPicker.value = true;
}

async function addTag(tagId: number) {
    error.value = null;
    saving.value = true;

    // Only add to individual tags (label tags are preserved server-side)
    const newIndividualIds = [...individualTags.value.map((t) => t.id), tagId];

    try {
        const tags = await setMarkerTags(props.markerId, newIndividualIds);
        markerTags.value = tags || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to add tag';
    } finally {
        saving.value = false;
    }
}

async function removeTag(tagId: number) {
    error.value = null;
    saving.value = true;

    const newIndividualIds = individualTags.value.filter((t) => t.id !== tagId).map((t) => t.id);

    try {
        const tags = await setMarkerTags(props.markerId, newIndividualIds);
        markerTags.value = tags || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to remove tag';
    } finally {
        saving.value = false;
    }
}

const reload = () => loadMarkerTags();
defineExpose({ reload });
</script>

<template>
    <div class="px-2 pb-2">
        <!-- Error -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 mb-1.5 flex items-center gap-1 rounded border px-2 py-1"
        >
            <Icon name="heroicons:exclamation-triangle" size="10" class="text-lava shrink-0" />
            <span class="truncate text-[9px] text-red-300">{{ error }}</span>
        </div>

        <div v-if="loading" class="flex items-center gap-1 py-1">
            <Icon name="svg-spinners:ring-resize" size="10" class="text-dim" />
            <span class="text-dim text-[9px]">Loading...</span>
        </div>

        <div v-else class="flex flex-wrap items-center gap-1">
            <!-- Label-derived tags (with lock icon, not removable) -->
            <span
                v-for="tag in labelTags"
                :key="'label-' + tag.id"
                class="flex items-center gap-1 rounded-full border px-1.5 py-px text-[9px]
                    font-medium text-white/80"
                :style="{
                    borderColor: tag.color + '40',
                    backgroundColor: tag.color + '10',
                }"
                :title="`From label default page)`"
            >
                <span
                    class="inline-block h-1.5 w-1.5 rounded-full"
                    :style="{ backgroundColor: tag.color }"
                />
                {{ tag.name }}
                <Icon name="heroicons:lock-closed" size="8" class="text-dim/60" />
            </span>

            <!-- Individual tags (removable) -->
            <span
                v-for="tag in individualTags"
                :key="'individual-' + tag.id"
                class="group flex items-center gap-1 rounded-full border px-1.5 py-px text-[9px]
                    font-medium text-white"
                :style="{
                    borderColor: tag.color + '50',
                    backgroundColor: tag.color + '15',
                }"
            >
                <span
                    class="inline-block h-1.5 w-1.5 rounded-full"
                    :style="{ backgroundColor: tag.color }"
                />
                {{ tag.name }}
                <span
                    @click.stop="removeTag(tag.id)"
                    class="cursor-pointer opacity-0 transition-opacity group-hover:opacity-60
                        hover:opacity-100!"
                    :class="{ 'pointer-events-none': saving }"
                >
                    <Icon name="heroicons:x-mark" size="8" />
                </span>
            </span>

            <!-- Add tag button -->
            <button
                ref="anchorRef"
                @click.stop="onAddTagClick"
                class="border-border hover:border-border-hover flex h-4 w-4 items-center
                    justify-center rounded-full border transition-colors"
                :disabled="loadingAllTags || saving"
                title="Add tag"
            >
                <Icon
                    v-if="loadingAllTags || saving"
                    name="svg-spinners:ring-resize"
                    size="8"
                    class="text-dim"
                />
                <Icon v-else name="heroicons:plus" size="8" class="text-dim" />
            </button>

            <WatchTagPicker
                :visible="showTagPicker"
                :tags="availableTags"
                :anchor-el="anchorRef"
                @select="addTag"
                @close="showTagPicker = false"
            />
        </div>
    </div>
</template>
