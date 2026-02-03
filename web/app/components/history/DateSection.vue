<script setup lang="ts">
import type { WatchHistoryEntry } from '~/types/watch';

const props = defineProps<{
    dateKey: string;
    dateLabel: string;
    entries: WatchHistoryEntry[];
    entryCount: number;
}>();

const { formatDuration } = useFormatter();

const getProgress = (entry: WatchHistoryEntry) => {
    if (entry.watch.completed || !entry.scene?.duration) return undefined;
    return {
        last_position: entry.watch.last_position,
        duration: entry.scene.duration,
    };
};

const validEntries = computed(() => props.entries.filter((e) => e.scene));
</script>

<template>
    <div :id="`date-${dateKey}`" class="scroll-mt-4">
        <div class="mb-3 flex items-center gap-2">
            <h2 class="text-sm font-medium text-white">{{ dateLabel }}</h2>
            <span
                class="border-border bg-surface text-dim rounded-full border px-2 py-0.5 font-mono
                    text-[10px]"
            >
                {{ entryCount }}
            </span>
        </div>

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
            <SceneCard
                v-for="entry in validEntries"
                :key="entry.watch.id"
                :scene="entry.scene!"
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
            </SceneCard>
        </div>
    </div>
</template>
