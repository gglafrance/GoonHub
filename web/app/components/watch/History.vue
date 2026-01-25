<script setup lang="ts">
import type { UserVideoWatch } from '~/types/watch';

const route = useRoute();
const { getVideoWatchHistory } = useApi();
const { formatDuration } = useFormatter();
const seekToTime = inject<(time: number) => void>('seekToTime');

const videoId = computed(() => parseInt(route.params.id as string));

const handleResumeFromWatch = (position: number) => {
    if (seekToTime && position > 0) {
        seekToTime(position);
    }
};

const loading = ref(true);
const watches = ref<UserVideoWatch[]>([]);

const loadHistory = async () => {
    loading.value = true;
    try {
        const data = await getVideoWatchHistory(videoId.value, 20);
        watches.value = data.watches || [];
    } catch {
        // Non-critical, just show empty
    } finally {
        loading.value = false;
    }
};

onMounted(() => {
    loadHistory();
});
</script>

<template>
    <div class="space-y-4">
        <!-- Loading state -->
        <div v-if="loading" class="text-dim py-4 text-center text-[11px]">Loading...</div>

        <!-- Empty state -->
        <div v-else-if="watches.length === 0" class="text-dim py-4 text-center text-[11px]">
            No watch history for this video
        </div>

        <!-- Watch history list -->
        <div v-else class="space-y-2">
            <div
                v-for="watch in watches"
                :key="watch.id"
                class="border-border bg-surface flex items-center justify-between rounded-lg border
                    p-3"
            >
                <div class="flex items-center gap-3">
                    <div
                        class="border-border bg-panel flex h-8 w-8 items-center justify-center
                            rounded-md border"
                    >
                        <Icon
                            :name="watch.completed ? 'heroicons:check-circle' : 'heroicons:play'"
                            size="16"
                            :class="watch.completed ? 'text-emerald' : 'text-lava'"
                        />
                    </div>
                    <div>
                        <div class="text-xs text-white">
                            Watched for {{ formatDuration(watch.watch_duration) }}
                        </div>
                        <div class="text-dim mt-0.5 font-mono text-[10px]">
                            <NuxtTime :datetime="new Date(watch.watched_at)" relative />
                        </div>
                    </div>
                </div>

                <div class="flex items-center gap-3">
                    <div v-if="watch.last_position > 0" class="flex items-center gap-2">
                        <div class="text-right">
                            <div class="text-dim text-[10px]">Last position</div>
                            <div class="font-mono text-xs text-white">
                                {{ formatDuration(watch.last_position) }}
                            </div>
                        </div>
                        <button
                            class="border-lava/30 bg-lava/10 hover:bg-lava/20 hover:border-lava/50
                                flex h-7 w-7 items-center justify-center rounded-md border
                                transition-all"
                            title="Resume from this position"
                            @click="handleResumeFromWatch(watch.last_position)"
                        >
                            <Icon name="heroicons:play" size="12" class="text-lava" />
                        </button>
                    </div>

                    <span
                        v-if="watch.completed"
                        class="rounded-full border border-emerald-500/30 bg-emerald-500/15 px-2
                            py-0.5 text-[10px] font-medium text-emerald-400"
                    >
                        Completed
                    </span>
                </div>
            </div>
        </div>
    </div>
</template>
