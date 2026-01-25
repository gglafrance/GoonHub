<script setup lang="ts">
import type { WatchHistoryEntry } from '~/types/watch';

const { getUserWatchHistory } = useApi();
const { formatDuration } = useFormatter();

const entries = ref<WatchHistoryEntry[]>([]);
const isLoading = ref(true);
const total = ref(0);
const page = ref(1);
const limit = 20;

const loadHistory = async (newPage = 1) => {
    isLoading.value = true;
    try {
        const data = await getUserWatchHistory(newPage, limit);
        entries.value = data.entries || [];
        total.value = data.total || 0;
        page.value = newPage;
    } catch {
        entries.value = [];
    } finally {
        isLoading.value = false;
    }
};

const getThumbnailUrl = (entry: WatchHistoryEntry): string | null => {
    if (!entry.video?.thumbnail_path) return null;
    return `/thumbnails/${entry.video.id}`;
};

const getProgressPercentage = (entry: WatchHistoryEntry): number => {
    if (!entry.video?.duration || entry.video.duration === 0) return 0;
    if (entry.watch.completed) return 100;
    return Math.min((entry.watch.last_position / entry.video.duration) * 100, 100);
};

watch(
    () => page.value,
    (newPage) => {
        loadHistory(newPage);
    },
);

onMounted(() => {
    loadHistory();
});

definePageMeta({
    title: 'Watch History - GoonHub',
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-400">
            <!-- Header -->
            <div class="mb-6 flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <div
                        class="border-border bg-panel flex h-10 w-10 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:clock" size="20" class="text-lava" />
                    </div>
                    <div>
                        <h1 class="text-lg font-semibold text-white">Watch History</h1>
                        <p class="text-dim text-xs">Videos you've watched</p>
                    </div>
                </div>
                <span
                    class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                        font-mono text-[11px]"
                >
                    {{ total }} videos
                </span>
            </div>

            <!-- Loading State -->
            <div
                v-if="isLoading && entries.length === 0"
                class="flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading history..." />
            </div>

            <!-- Empty State -->
            <div
                v-else-if="entries.length === 0"
                class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                    border border-dashed text-center"
            >
                <div
                    class="bg-panel border-border flex h-10 w-10 items-center justify-center
                        rounded-lg border"
                >
                    <Icon name="heroicons:clock" size="20" class="text-dim" />
                </div>
                <p class="text-muted mt-3 text-sm">No watch history</p>
                <p class="text-dim mt-1 text-xs">Videos you watch will appear here</p>
                <NuxtLink
                    to="/"
                    class="border-border bg-surface text-muted hover:border-border-hover mt-4
                        rounded-lg border px-4 py-2 text-xs font-medium transition-all
                        hover:text-white"
                >
                    Browse Library
                </NuxtLink>
            </div>

            <!-- History Grid -->
            <div v-else>
                <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                    <NuxtLink
                        v-for="entry in entries"
                        :key="entry.watch.id"
                        :to="`/watch/${entry.watch.video_id}`"
                        class="group border-border bg-surface hover:border-border-hover
                            hover:bg-elevated relative block overflow-hidden rounded-lg border
                            transition-all duration-200"
                    >
                        <div class="bg-void relative aspect-video w-full">
                            <img
                                v-if="getThumbnailUrl(entry)"
                                :src="getThumbnailUrl(entry)!"
                                class="absolute inset-0 h-full w-full object-contain
                                    transition-transform duration-300 group-hover:scale-[1.03]"
                                :alt="entry.video?.title || 'Video'"
                                loading="lazy"
                            />

                            <div
                                v-else
                                class="text-dim group-hover:text-lava absolute inset-0 flex
                                    items-center justify-center transition-colors"
                            >
                                <Icon name="heroicons:play" size="32" />
                            </div>

                            <!-- Duration badge -->
                            <div
                                v-if="entry.video?.duration && entry.video.duration > 0"
                                class="bg-void/90 absolute right-1.5 bottom-1.5 rounded px-1.5
                                    py-0.5 font-mono text-[10px] font-medium text-white
                                    backdrop-blur-sm"
                            >
                                {{ formatDuration(entry.video.duration) }}
                            </div>

                            <!-- Completed badge -->
                            <div
                                v-if="entry.watch.completed"
                                class="absolute top-1.5 right-1.5 rounded bg-emerald-500/90 px-1.5
                                    py-0.5 text-[9px] font-semibold text-white backdrop-blur-sm"
                            >
                                Watched
                            </div>

                            <!-- Progress bar -->
                            <div
                                v-if="!entry.watch.completed && getProgressPercentage(entry) > 0"
                                class="absolute right-0 bottom-0 left-0 h-0.5 bg-white/20"
                            >
                                <div
                                    class="bg-lava h-full"
                                    :style="{ width: `${getProgressPercentage(entry)}%` }"
                                ></div>
                            </div>

                            <!-- Hover overlay -->
                            <div
                                class="bg-lava/0 group-hover:bg-lava/5 absolute inset-0
                                    transition-colors duration-200"
                            ></div>
                        </div>

                        <div class="p-3">
                            <h3
                                class="truncate text-xs font-medium text-white/90 transition-colors
                                    group-hover:text-white"
                                :title="entry.video?.title"
                            >
                                {{ entry.video?.title || 'Unknown Video' }}
                            </h3>
                            <div
                                class="text-dim mt-1.5 flex items-center justify-between font-mono
                                    text-[10px]"
                            >
                                <NuxtTime :datetime="new Date(entry.watch.watched_at)" relative />
                                <span
                                    v-if="!entry.watch.completed && entry.watch.last_position > 0"
                                >
                                    {{ formatDuration(entry.watch.last_position) }}
                                </span>
                            </div>
                        </div>
                    </NuxtLink>
                </div>

                <Pagination v-model="page" :total="total" :limit="limit" />
            </div>
        </div>
    </div>
</template>
