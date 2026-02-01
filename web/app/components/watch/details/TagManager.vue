<script setup lang="ts">
import type { Tag } from '~/types/tag';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

const props = defineProps<{
    sceneId: number;
}>();

const router = useRouter();
const { fetchTags, setSceneTags } = useApiTags();

function navigateToTagSearch(tagName: string) {
    router.push({ path: '/search', query: { tags: tagName } });
}

// Inject centralized watch page data
const watchPageData = inject<WatchPageData>(WATCH_PAGE_DATA_KEY);

const error = ref<string | null>(null);
const allTags = ref<Tag[]>([]);
const allTagsLoaded = ref(false);
const loadingAllTags = ref(false);
const showTagPicker = ref(false);
const anchorRef = ref<HTMLElement | null>(null);

// Use centralized data for loading state and scene tags
const loading = computed(() => watchPageData?.loading.details ?? false);
const sceneTags = computed(() => watchPageData?.tags.value ?? []);

const availableTags = computed(() =>
    allTags.value.filter((t) => !sceneTags.value.some((st) => st.id === t.id)),
);

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

    const newIds = [...sceneTags.value.map((t) => t.id), tagId];

    try {
        const res = await setSceneTags(props.sceneId, newIds);
        // Update centralized data
        watchPageData?.setTags(res.data || []);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    }
}

async function removeTag(tagId: number) {
    error.value = null;

    const newIds = sceneTags.value.filter((t) => t.id !== tagId).map((t) => t.id);

    try {
        const res = await setSceneTags(props.sceneId, newIds);
        // Update centralized data
        watchPageData?.setTags(res.data || []);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    }
}

// Expose reload method for parent to call after metadata update
const reload = () => watchPageData?.refreshTags();
defineExpose({ reload });
</script>

<template>
    <div class="space-y-2">
        <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Tags</h3>

        <!-- Error -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-red-300">{{ error }}</span>
        </div>

        <div v-if="loading" class="flex items-center gap-2 py-2">
            <LoadingSpinner />
        </div>

        <div v-else class="flex flex-wrap items-center gap-1.5">
            <!-- Applied tags -->
            <span
                v-for="tag in sceneTags"
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
                <span
                    @click="navigateToTagSearch(tag.name)"
                    class="cursor-pointer transition-opacity hover:opacity-80"
                >
                    {{ tag.name }}
                </span>
                <span
                    @click="removeTag(tag.id)"
                    class="cursor-pointer opacity-0 transition-opacity group-hover:opacity-60
                        hover:opacity-100!"
                >
                    <Icon name="heroicons:x-mark" size="10" />
                </span>
            </span>

            <!-- Add tag button -->
            <button
                ref="anchorRef"
                @click="onAddTagClick"
                class="border-border hover:border-border-hover flex h-5 w-5 items-center
                    justify-center rounded-full border transition-colors"
                :disabled="loadingAllTags"
                title="Add tag"
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
