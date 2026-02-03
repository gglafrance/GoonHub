<script setup lang="ts">
import type { WatchHistoryEntry, DailyActivityCount, DateGroup } from '~/types/watch';

const { getUserWatchHistoryByDateRange, getDailyActivity } = useApiScenes();

useHead({ title: 'Watch History' });

useSeoMeta({
    title: 'Watch History',
    ogTitle: 'Watch History - GoonHub',
    description: 'Your recently watched scenes',
    ogDescription: 'Your recently watched scenes',
});

const rangeDays = ref(30);
const entries = ref<WatchHistoryEntry[]>([]);
const activityCounts = ref<DailyActivityCount[]>([]);
const isLoading = ref(true);

const totalScenes = computed(() => {
    const seen = new Set<number>();
    for (const e of entries.value) {
        if (e.scene) seen.add(e.scene.id);
    }
    return seen.size;
});

const formatDateLabel = (dateKey: string) => {
    const today = new Date();
    const todayKey =
        today.getFullYear() +
        '-' +
        String(today.getMonth() + 1).padStart(2, '0') +
        '-' +
        String(today.getDate()).padStart(2, '0');

    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);
    const yesterdayKey =
        yesterday.getFullYear() +
        '-' +
        String(yesterday.getMonth() + 1).padStart(2, '0') +
        '-' +
        String(yesterday.getDate()).padStart(2, '0');

    if (dateKey === todayKey) return 'Today';
    if (dateKey === yesterdayKey) return 'Yesterday';

    const d = new Date(dateKey + 'T00:00:00');
    return d.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
};

const dateGroups = computed<DateGroup[]>(() => {
    const groups = new Map<string, WatchHistoryEntry[]>();

    for (const entry of entries.value) {
        const d = new Date(entry.watch.watched_at);
        const key =
            d.getFullYear() +
            '-' +
            String(d.getMonth() + 1).padStart(2, '0') +
            '-' +
            String(d.getDate()).padStart(2, '0');

        if (!groups.has(key)) {
            groups.set(key, []);
        }
        groups.get(key)!.push(entry);
    }

    return Array.from(groups.entries()).map(([dateKey, groupEntries]) => ({
        dateKey,
        date: formatDateLabel(dateKey),
        entries: groupEntries,
    }));
});

const loadData = async () => {
    isLoading.value = true;
    try {
        const [historyData, activityData] = await Promise.all([
            getUserWatchHistoryByDateRange(rangeDays.value, 2000),
            getDailyActivity(rangeDays.value),
        ]);
        entries.value = historyData.entries || [];
        activityCounts.value = activityData.counts || [];
    } catch {
        entries.value = [];
        activityCounts.value = [];
    } finally {
        isLoading.value = false;
    }
};

const scrollToDate = (dateKey: string) => {
    const el = document.getElementById('date-' + dateKey);
    if (el) {
        el.scrollIntoView({ behavior: 'smooth' });
    }
};

watch(rangeDays, () => {
    loadData();
});

onMounted(() => {
    loadData();
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
                        <p class="text-dim text-xs">Scenes you've watched</p>
                    </div>
                </div>
                <span
                    class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                        font-mono text-[11px]"
                >
                    {{ totalScenes }} scenes
                </span>
            </div>

            <!-- Range Selector -->
            <div class="mb-4">
                <HistoryRangeSelector v-model="rangeDays" />
            </div>

            <!-- Activity Chart -->
            <div class="border-border bg-panel mb-6 rounded-xl border p-3">
                <div class="text-dim mb-2 text-[11px] font-medium uppercase tracking-wider">
                    Activity
                </div>
                <HistoryActivityChart
                    :counts="activityCounts"
                    :range-days="rangeDays"
                    :is-loading="isLoading"
                    @bar-click="scrollToDate"
                />
            </div>

            <!-- Loading State -->
            <div
                v-if="isLoading && entries.length === 0"
                class="flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading history..." />
            </div>

            <!-- Empty State -->
            <HistoryEmptyState v-else-if="entries.length === 0" />

            <!-- Date-grouped History -->
            <div v-else class="space-y-8">
                <HistoryDateSection
                    v-for="group in dateGroups"
                    :key="group.dateKey"
                    :date-key="group.dateKey"
                    :date-label="group.date"
                    :entries="group.entries"
                    :entry-count="group.entries.length"
                />
            </div>
        </div>
    </div>
</template>
