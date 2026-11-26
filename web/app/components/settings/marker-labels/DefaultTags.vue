<script setup lang="ts">
import type { MarkerLabelWithUsage } from '~/types/markerLabel';
import type { Tag } from '~/types/tag';

// Default tags of a marker label (the same for every user): every marker of the label gets
// them as tags "from label".
const props = defineProps<{
    label: MarkerLabelWithUsage;
}>();

const emit = defineEmits<{
    updated: [tags: Tag[]];
}>();

const { setMarkerLabelTags } = useApiMarkerLabels();
const { fetchTags } = useApiTags();

const tags = ref<Tag[]>([...props.label.default_tags]);
const allTags = ref<Tag[]>([]);
const loadingAll = ref(false);
const saving = ref(false);
const error = ref('');
const showPicker = ref(false);
const anchorRef = ref<HTMLElement | null>(null);

const available = computed(() =>
    allTags.value.filter((t) => !tags.value.some((d) => d.id === t.id)),
);

async function openPicker() {
    if (showPicker.value) {
        showPicker.value = false;
        return;
    }
    if (allTags.value.length === 0 && !loadingAll.value) {
        loadingAll.value = true;
        try {
            const res = await fetchTags();
            allTags.value = res.data || [];
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to load tags';
        } finally {
            loadingAll.value = false;
        }
    }
    showPicker.value = true;
}

async function save(ids: number[]) {
    saving.value = true;
    error.value = '';
    try {
        tags.value = await setMarkerLabelTags(props.label.id, ids);
        emit('updated', tags.value);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update the default tags';
    } finally {
        saving.value = false;
    }
}

const add = (id: number) => save([...tags.value.map((t) => t.id), id]);
const remove = (id: number) => save(tags.value.filter((t) => t.id !== id).map((t) => t.id));
</script>

<template>
    <div>
        <label class="text-dim mb-1 block text-[10px] font-semibold tracking-wider uppercase">
            Default tags
        </label>
        <p class="text-dim mb-2 text-[11px]">
            Added to every marker with this label, for every user.
        </p>
        <div class="flex flex-wrap items-center gap-1.5">
            <span
                v-for="tag in tags"
                :key="tag.id"
                class="group/tag flex items-center gap-1.5 rounded-full border px-2.5 py-0.5
                    text-[11px] font-medium text-white"
                :style="{ borderColor: tag.color + '60', backgroundColor: tag.color + '15' }"
            >
                <span
                    class="inline-block h-2 w-2 rounded-full"
                    :style="{ backgroundColor: tag.color }"
                />
                {{ tag.name }}
                <button
                    class="opacity-60 transition-opacity hover:opacity-100"
                    :disabled="saving"
                    :aria-label="`Remove ${tag.name}`"
                    @click="remove(tag.id)"
                >
                    <Icon name="heroicons:x-mark" size="10" />
                </button>
            </span>
            <span v-if="tags.length === 0" class="text-dim text-[11px] italic">
                No default tags
            </span>
            <button
                ref="anchorRef"
                class="border-border hover:border-border-hover flex h-5 w-5 items-center
                    justify-center rounded-full border transition-colors"
                :disabled="loadingAll || saving"
                title="Add a default tag"
                @click="openPicker"
            >
                <Icon
                    v-if="loadingAll || saving"
                    name="svg-spinners:ring-resize"
                    size="12"
                    class="text-dim"
                />
                <Icon v-else name="heroicons:plus" size="12" class="text-dim" />
            </button>
            <WatchTagPicker
                :visible="showPicker"
                :tags="available"
                :anchor-el="anchorRef"
                @select="add"
                @close="showPicker = false"
            />
        </div>
        <p v-if="error" class="text-lava mt-1 text-[11px]">{{ error }}</p>
    </div>
</template>
