<script setup lang="ts">
import type { WatchHistoryEntry } from '~/types/watch';

const { getUserWatchHistory } = useApi();
const { formatDuration } = useFormatter();

useHead({ title: 'Watch History' });

const entries = ref<WatchHistoryEntry[]>([]);
const isLoading = ref(true);
const total = ref(0);
const page = ref(1);
const limit = 20;

// Filter out entries where video was deleted
const validEntries = computed(() => entries.value.filter((e) => e.video));

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

const getProgress = (entry: WatchHistoryEntry) => {
    if (entry.watch.completed || !entry.video?.duration) return undefined;
    return {
        last_position: entry.watch.last_position,
        duration: entry.video.duration,
    };
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
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
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
                    <VideoCard
                        v-for="entry in validEntries"
                        :key="entry.watch.id"
                        :video="entry.video!"
                        :progress="getProgress(entry)"
                        :completed="entry.watch.completed"
                        fluid
                    >
                        <template #footer>
                            <NuxtTime :datetime="new Date(entry.watch.watched_at)" relative />
                            <span v-if="!entry.watch.completed && entry.watch.last_position > 0">
                                {{ formatDuration(entry.watch.last_position) }}
                            </span>
                        </template>
                    </VideoCard>
                </div>

                <Pagination v-model="page" :total="total" :limit="limit" />
            </div>
        </div>
    </div>
</template>
