<script setup lang="ts">
import type { ShortItem } from '~/types/shorts';
import type { ShortEditor } from '~/composables/useShortEditor';
import { SHORT_EDITOR_KEY, formatShortClock, formatShortSpan } from '~/composables/useShortEditor';

// Shorts tab: create a short from this scene and list the ones already cut.
const route = useRoute();
const { getSceneShorts } = useApiShorts();
const shortTasks = useShortTasksStore();
const editor = inject<ShortEditor>(SHORT_EDITOR_KEY);

const sceneId = computed(() => parseInt(route.params.id as string));

const shorts = ref<ShortItem[]>([]);
const total = ref(0);
const loading = ref(true);
const error = ref('');

const maxDuration = computed(() => editor?.maxDuration.value ?? 60);
const canCreate = computed(() => editor?.canCreate.value ?? false);
const tasks = computed(() => shortTasks.forSource(sceneId.value));

const load = async () => {
    const id = sceneId.value;
    error.value = '';
    try {
        const res = await getSceneShorts(id);
        if (id !== sceneId.value) return;
        shorts.value = res.data;
        total.value = res.total;
        // Pick up encodes started before this page was opened
        for (const task of res.pending) {
            if (!shortTasks.tasks[task.task_id]) shortTasks.upsert(task);
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load shorts';
    } finally {
        loading.value = false;
    }
};

onMounted(load);
watch(sceneId, () => {
    loading.value = true;
    shorts.value = [];
    load();
});
watch(
    () => shortTasks.completedTick[sceneId.value],
    () => load(),
);

// The feed filters on the stored duration; a new short has none until its
// metadata job runs, so fall back to the cut range until then.
const notInFeed = (s: ShortItem) => {
    const length =
        s.duration > 0 || s.source_start === null || s.source_end === null
            ? s.duration
            : s.source_end - s.source_start;
    return length > maxDuration.value;
};
</script>

<template>
    <div class="space-y-4">
        <WatchShortsEditor />

        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-white">{{ error }}</span>
        </div>

        <div v-if="loading" class="text-dim py-4 text-center text-[11px]">Loading shorts...</div>

        <div
            v-else-if="shorts.length === 0 && tasks.length === 0"
            class="text-dim py-4 text-center text-[11px]"
        >
            <template v-if="canCreate">
                No shorts yet. Click + Add short to make one from this video.
            </template>
            <template v-else>No shorts have been made from this video.</template>
        </div>

        <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
            <WatchShortsTaskCard
                v-for="task in tasks"
                :key="task.task_id"
                :task="task"
                @dismiss="shortTasks.remove"
            />
            <div v-for="s in shorts" :key="s.id" class="min-w-0 space-y-1">
                <SceneCard :scene="s" fluid />
                <div class="flex items-center gap-1.5 px-0.5 font-mono text-[10px]">
                    <span v-if="s.source_start !== null && s.source_end !== null" class="text-dim">
                        {{ formatShortClock(s.source_start) }}-{{
                            formatShortClock(s.source_end)
                        }}
                        · {{ formatShortSpan(s.source_end - s.source_start) }}
                    </span>
                    <span
                        v-if="notInFeed(s)"
                        class="ml-auto rounded-full border border-amber-500/30 bg-amber-500/10
                            px-1.5 py-px font-sans text-[9px] font-medium text-amber-300"
                        :title="`Longer than the ${formatShortSpan(maxDuration)} Shorts limit`"
                    >
                        Not in feed
                    </span>
                </div>
            </div>
        </div>
    </div>
</template>
