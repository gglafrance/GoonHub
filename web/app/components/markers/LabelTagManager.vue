<script setup lang="ts">
import type { Tag } from '~/types/tag';

const props = defineProps<{
    label: string;
}>();

const emit = defineEmits<{
    (e: 'updated'): void;
}>();

const { fetchLabelTags, setLabelTags } = useApiMarkers();
const { fetchTags } = useApiTags();

const loading = ref(false);
const saving = ref(false);
const error = ref<string | null>(null);
const allTags = ref<Tag[]>([]);
const allTagsLoaded = ref(false);
const loadingAllTags = ref(false);
const labelTags = ref<Tag[]>([]);
const showTagPicker = ref(false);
const anchorRef = ref<HTMLElement | null>(null);

const availableTags = computed(() =>
    allTags.value.filter((t) => !labelTags.value.some((lt) => lt.id === t.id)),
);

onMounted(() => {
    loadLabelTags();
});

watch(
    () => props.label,
    () => {
        loadLabelTags();
    },
);

async function loadLabelTags() {
    loading.value = true;
    error.value = null;

    try {
        labelTags.value = await fetchLabelTags(props.label);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load label tags';
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

    const newIds = [...labelTags.value.map((t) => t.id), tagId];

    try {
        labelTags.value = await setLabelTags(props.label, newIds);
        emit('updated');
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    } finally {
        saving.value = false;
    }
}

async function removeTag(tagId: number) {
    error.value = null;
    saving.value = true;

    const newIds = labelTags.value.filter((t) => t.id !== tagId).map((t) => t.id);

    try {
        labelTags.value = await setLabelTags(props.label, newIds);
        emit('updated');
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    } finally {
        saving.value = false;
    }
}

const reload = () => loadLabelTags();
defineExpose({ reload });
</script>

<template>
    <div class="border-border bg-surface rounded-xl border p-4">
        <div class="mb-3 flex items-start justify-between">
            <div>
                <h3 class="text-sm font-medium text-white">Default Tags</h3>
                <p class="text-dim mt-0.5 text-[11px]">
                    These tags are automatically applied to all markers with this label.
                </p>
            </div>
            <div v-if="saving" class="text-dim flex items-center gap-1 text-[10px]">
                <Icon name="svg-spinners:ring-resize" size="12" />
                Saving...
            </div>
        </div>

        <!-- Error -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 mb-3 flex items-center gap-2 rounded-lg border px-3
                py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-red-300">{{ error }}</span>
            <button class="text-dim ml-auto text-xs hover:text-white" @click="error = null">
                Dismiss
            </button>
        </div>

        <div v-if="loading" class="flex items-center gap-2 py-2">
            <LoadingSpinner />
        </div>

        <div v-else class="flex flex-wrap items-center gap-1.5">
            <!-- Applied tags -->
            <span
                v-for="tag in labelTags"
                :key="tag.id"
                class="group flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px]
                    font-medium text-white"
                :style="{
                    borderColor: tag.color + '60',
                    backgroundColor: tag.color + '15',
                }"
            >
                <span
                    class="inline-block h-2 w-2 rounded-full"
                    :style="{ backgroundColor: tag.color }"
                />
                {{ tag.name }}
                <span
                    class="cursor-pointer opacity-0 transition-opacity group-hover:opacity-60
                        hover:opacity-100!"
                    :class="{ 'pointer-events-none': saving }"
                    @click="removeTag(tag.id)"
                >
                    <Icon name="heroicons:x-mark" size="10" />
                </span>
            </span>

            <!-- Empty state -->
            <span v-if="labelTags.length === 0" class="text-dim text-[11px] italic">
                No default tags set
            </span>

            <!-- Add tag button -->
            <button
                ref="anchorRef"
                class="border-border hover:border-border-hover flex h-5 w-5 items-center
                    justify-center rounded-full border transition-colors"
                :disabled="loadingAllTags || saving"
                title="Add tag"
                @click="onAddTagClick"
            >
                <Icon
                    v-if="loadingAllTags"
                    name="heroicons:arrow-path"
                    size="12"
                    class="text-dim animate-spin"
                />
                <Icon v-else name="heroicons:plus" size="12" class="text-dim" />
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
