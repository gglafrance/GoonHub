<script setup lang="ts">
import type { Video } from '~/types/video';
import type { Tag } from '~/types/tag';

const video = inject<Ref<Video | null>>('watchVideo');
const { fetchTags, fetchVideoTags, setVideoTags } = useApi();

const loading = ref(false);
const error = ref<string | null>(null);

const allTags = ref<Tag[]>([]);
const videoTags = ref<Tag[]>([]);
const showTagPicker = ref(false);

const anchorRef = ref<HTMLElement | null>(null);

const availableTags = computed(() =>
    allTags.value.filter((t) => !videoTags.value.some((vt) => vt.id === t.id)),
);

onMounted(async () => {
    await loadTags();
});

async function loadTags() {
    if (!video?.value) return;
    loading.value = true;
    error.value = null;

    try {
        const [tagsRes, videoTagsRes] = await Promise.all([
            fetchTags(),
            fetchVideoTags(video.value.id),
        ]);
        allTags.value = tagsRes.data || [];
        videoTags.value = videoTagsRes.data || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load tags';
    } finally {
        loading.value = false;
    }
}

async function addTag(tagId: number) {
    if (!video?.value) return;
    error.value = null;

    const newIds = [...videoTags.value.map((t) => t.id), tagId];

    try {
        const res = await setVideoTags(video.value.id, newIds);
        videoTags.value = res.data || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    }
}

async function removeTag(tagId: number) {
    if (!video?.value) return;
    error.value = null;

    const newIds = videoTags.value.filter((t) => t.id !== tagId).map((t) => t.id);

    try {
        const res = await setVideoTags(video.value.id, newIds);
        videoTags.value = res.data || [];
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
    }
}
</script>

<template>
    <div class="min-h-64 space-y-4">
        <!-- Error -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-red-300">{{ error }}</span>
        </div>

        <!-- Tags section -->
        <div class="space-y-2">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Tags</h3>

            <div v-if="loading" class="flex items-center gap-2 py-2">
                <LoadingSpinner />
            </div>

            <div v-else class="flex flex-wrap items-center gap-1.5">
                <!-- Applied tags -->
                <span
                    v-for="tag in videoTags"
                    :key="tag.id"
                    class="group flex items-center gap-1.5 rounded-full border px-2.5 py-0.5
                        text-[11px] font-medium text-white"
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
                    @click="showTagPicker = !showTagPicker"
                    class="border-border hover:border-border-hover flex h-5 w-5 items-center
                        justify-center rounded-full border transition-colors"
                    title="Add tag"
                >
                    <Icon name="heroicons:plus" size="12" class="text-dim" />
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
    </div>
</template>
