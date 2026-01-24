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

const pickerRef = ref<HTMLElement | null>(null);
const searchQuery = ref('');
const sortMode = ref<'az' | 'za' | 'most' | 'least'>('az');

const sortLabels: Record<string, string> = {
    az: 'A-Z',
    za: 'Z-A',
    most: 'Most used',
    least: 'Least used',
};

function cycleSortMode() {
    const modes: Array<'az' | 'za' | 'most' | 'least'> = ['az', 'za', 'most', 'least'];
    const idx = modes.indexOf(sortMode.value);
    sortMode.value = modes[(idx + 1) % modes.length];
}

const availableTags = computed(() => {
    let tags = allTags.value.filter((t) => !videoTags.value.some((vt) => vt.id === t.id));

    if (searchQuery.value) {
        const q = searchQuery.value.toLowerCase();
        tags = tags.filter((t) => t.name.toLowerCase().includes(q));
    }

    switch (sortMode.value) {
        case 'az':
            tags.sort((a, b) => a.name.localeCompare(b.name));
            break;
        case 'za':
            tags.sort((a, b) => b.name.localeCompare(a.name));
            break;
        case 'most':
            tags.sort((a, b) => b.video_count - a.video_count || a.name.localeCompare(b.name));
            break;
        case 'least':
            tags.sort((a, b) => a.video_count - b.video_count || a.name.localeCompare(b.name));
            break;
    }

    return tags;
});

onMounted(async () => {
    await loadTags();
});

function onClickOutside(event: MouseEvent) {
    if (pickerRef.value && !pickerRef.value.contains(event.target as Node)) {
        showTagPicker.value = false;
    }
}

watch(showTagPicker, (open) => {
    if (open) {
        setTimeout(() => document.addEventListener('click', onClickOutside), 0);
    } else {
        document.removeEventListener('click', onClickOutside);
        searchQuery.value = '';
    }
});

onBeforeUnmount(() => {
    document.removeEventListener('click', onClickOutside);
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
                <div class="relative" ref="pickerRef">
                    <button
                        @click="showTagPicker = !showTagPicker"
                        class="border-border hover:border-border-hover flex h-5 w-5 items-center
                            justify-center rounded-full border transition-colors"
                        title="Add tag"
                    >
                        <Icon name="heroicons:plus" size="12" class="text-dim" />
                    </button>

                    <!-- Tag picker dropdown -->
                    <div
                        v-if="showTagPicker"
                        class="border-border bg-panel absolute top-full left-0 z-50 mt-1.5 min-w-48
                            rounded-lg border shadow-xl"
                    >
                        <!-- Search + Sort header -->
                        <div class="border-border/50 flex items-center gap-1 border-b px-2 py-1.5">
                            <input
                                v-model="searchQuery"
                                type="text"
                                placeholder="Search tags..."
                                class="bg-transparent text-[11px] text-white/80 placeholder-white/30
                                    outline-none flex-1 min-w-0"
                                @click.stop
                            />
                            <button
                                @click.stop="cycleSortMode()"
                                class="text-dim hover:text-white/80 shrink-0 rounded px-1.5 py-0.5
                                    text-[9px] transition-colors hover:bg-white/5"
                                :title="`Sort: ${sortLabels[sortMode]}`"
                            >
                                {{ sortLabels[sortMode] }}
                            </button>
                        </div>

                        <div v-if="availableTags.length === 0" class="px-3 py-2">
                            <p class="text-dim text-[11px]">
                                {{ searchQuery ? 'No matching tags' : 'No more tags available' }}
                            </p>
                        </div>
                        <div v-else class="max-h-48 overflow-y-auto py-1">
                            <button
                                v-for="tag in availableTags"
                                :key="tag.id"
                                @click="addTag(tag.id)"
                                class="flex w-full items-center gap-2 px-3 py-1.5 text-left
                                    text-[11px] text-white/70 transition-colors hover:bg-white/5
                                    hover:text-white"
                            >
                                <span
                                    class="inline-block h-2 w-2 rounded-full shrink-0"
                                    :style="{ backgroundColor: tag.color }"
                                />
                                <span class="flex-1 truncate">{{ tag.name }}</span>
                                <span class="text-white/30 text-[10px]">({{ tag.video_count }})</span>
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
