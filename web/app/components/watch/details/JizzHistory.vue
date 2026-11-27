<script setup lang="ts">
// Wraps the jizz button: hovering shows the dated o of the scene (most recent first).
const props = defineProps<{
    sceneId: number;
    // Current count, used to refresh the history after an increment
    count: number;
}>();

interface JizzedResponse {
    count: number;
    history: { jizzed_at: string }[];
    history_total: number;
    last_jizzed_at: string | null;
}

const MAX_SHOWN = 20;

const { fetchJizzedCount } = useApiScenes();
const { formatDateTime } = useDateFormatter();

const trigger = ref<HTMLElement | null>(null);
const open = ref(false);
const loaded = ref(false);
const loading = ref(false);
const data = ref<JizzedResponse | null>(null);
const popoverStyle = ref<Record<string, string>>({});
let hideTimeout: ReturnType<typeof setTimeout> | null = null;

const load = async () => {
    if (loading.value) return;
    loading.value = true;
    try {
        data.value = (await fetchJizzedCount(props.sceneId)) as JizzedResponse;
        loaded.value = true;
    } catch {
        data.value = null;
    } finally {
        loading.value = false;
    }
};

const updatePosition = () => {
    if (!trigger.value) return;
    const rect = trigger.value.getBoundingClientRect();
    popoverStyle.value = {
        top: `${rect.bottom + 6}px`,
        left: `${Math.max(8, rect.left + rect.width / 2 - 110)}px`,
    };
};

const show = () => {
    if (hideTimeout) {
        clearTimeout(hideTimeout);
        hideTimeout = null;
    }
    updatePosition();
    open.value = true;
    if (!loaded.value) load();
};

const hide = () => {
    hideTimeout = setTimeout(() => {
        open.value = false;
    }, 150);
};

// After an increment, the cached history is stale
watch(
    () => props.count,
    () => {
        if (open.value) load();
        else loaded.value = false;
    },
);

watch(
    () => props.sceneId,
    () => {
        loaded.value = false;
        data.value = null;
    },
);

const shown = computed(() => (data.value?.history ?? []).slice(0, MAX_SHOWN));
const remaining = computed(() =>
    Math.max(0, (data.value?.history_total ?? 0) - shown.value.length),
);
const undated = computed(() =>
    Math.max(0, (data.value?.count ?? 0) - (data.value?.history_total ?? 0)),
);

const formatAbsolute = (iso: string) => formatDateTime(iso);

onBeforeUnmount(() => {
    if (hideTimeout) clearTimeout(hideTimeout);
});
</script>

<template>
    <div ref="trigger" class="relative" @mouseenter="show" @mouseleave="hide">
        <slot />

        <Teleport to="body">
            <Transition
                enter-active-class="transition-opacity duration-100"
                enter-from-class="opacity-0"
                leave-active-class="transition-opacity duration-75"
                leave-to-class="opacity-0"
            >
                <div
                    v-if="open && (loading || (data && data.count > 0))"
                    class="border-border bg-panel/95 fixed z-100 w-56 rounded-lg border p-2.5
                        shadow-lg backdrop-blur-md"
                    :style="popoverStyle"
                    @mouseenter="show"
                    @mouseleave="hide"
                >
                    <div v-if="loading && !data" class="text-dim text-[11px]">Loading...</div>
                    <template v-else-if="data">
                        <div class="mb-1.5 flex items-center justify-between text-[11px]">
                            <span class="text-white">History</span>
                            <span v-if="data.last_jizzed_at" class="text-dim">
                                last
                                <NuxtTime :datetime="data.last_jizzed_at" relative />
                            </span>
                        </div>
                        <ul class="max-h-56 space-y-0.5 overflow-y-auto">
                            <li
                                v-for="(event, index) in shown"
                                :key="event.jizzed_at + index"
                                class="flex items-center justify-between gap-2 text-[11px]"
                                :title="formatAbsolute(event.jizzed_at)"
                            >
                                <span class="text-dim">
                                    <NuxtTime :datetime="event.jizzed_at" relative />
                                </span>
                                <span class="font-mono text-[10px] text-white/50">
                                    {{ formatAbsolute(event.jizzed_at) }}
                                </span>
                            </li>
                        </ul>
                        <p v-if="remaining > 0" class="text-dim mt-1 text-[10px]">
                            and {{ remaining }} more
                        </p>
                        <p v-if="undated > 0" class="text-dim mt-1 text-[10px]">
                            {{ undated }} without a date
                        </p>
                    </template>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
