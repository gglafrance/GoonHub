<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import type { MarkerWidgetPayload, WidgetMarker } from '~/types/homepageMarkerWidgets';

// marker_compilation widget: an in-page player chaining marker clips. Each clip plays from the
// marker start to its end (or start + clip_seconds), then the next one starts.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const { formatMarkerRange } = useFormatter();

const payload = computed(() => props.data.widget as MarkerWidgetPayload | undefined);
const clipSeconds = computed(() => payload.value?.clip_seconds ?? 15);

const queue = ref<WidgetMarker[]>([]);
const index = ref(0);
const playing = ref(false);
const started = ref(false);

const videoEl = ref<HTMLVideoElement | null>(null);
const preloadEl = ref<HTMLVideoElement | null>(null);

watch(
    () => payload.value?.items,
    (items) => {
        queue.value = [...(items ?? [])];
        index.value = 0;
        started.value = false;
        playing.value = false;
    },
    { immediate: true },
);

const current = computed(() => queue.value[index.value] ?? null);
const next = computed(
    () => queue.value[(index.value + 1) % Math.max(queue.value.length, 1)] ?? null,
);

const streamUrl = (marker: WidgetMarker) => `/api/v1/scenes/${marker.scene_id}/stream`;

const clipEnd = (marker: WidgetMarker) =>
    marker.end_timestamp && marker.end_timestamp > marker.timestamp
        ? marker.end_timestamp
        : marker.timestamp + clipSeconds.value;

const clipProgress = ref(0);

// Loads the current clip: switch source when the scene changes, then seek once metadata is known
const loadCurrent = () => {
    const el = videoEl.value;
    const marker = current.value;
    if (!el || !marker) return;
    const src = streamUrl(marker);
    const seekAndPlay = () => {
        el.currentTime = marker.timestamp;
        if (playing.value) el.play().catch(() => (playing.value = false));
    };
    if (el.getAttribute('src') !== src) {
        el.setAttribute('src', src);
        el.addEventListener('loadedmetadata', seekAndPlay, { once: true });
        el.load();
    } else if (el.readyState >= 1) {
        seekAndPlay();
    } else {
        el.addEventListener('loadedmetadata', seekAndPlay, { once: true });
    }
    // Warm up the next scene (metadata only)
    if (preloadEl.value && next.value && next.value.scene_id !== marker.scene_id) {
        preloadEl.value.setAttribute('src', streamUrl(next.value));
    }
};

const goTo = (i: number) => {
    if (queue.value.length === 0) return;
    index.value = (i + queue.value.length) % queue.value.length;
    clipProgress.value = 0;
    loadCurrent();
};

const nextClip = () => goTo(index.value + 1);
const previousClip = () => goTo(index.value - 1);

const togglePlay = () => {
    const el = videoEl.value;
    if (!el || !current.value) return;
    if (!started.value) {
        started.value = true;
        playing.value = true;
        loadCurrent();
        return;
    }
    if (el.paused) {
        playing.value = true;
        el.play().catch(() => (playing.value = false));
    } else {
        playing.value = false;
        el.pause();
    }
};

const shuffle = () => {
    const items = [...queue.value];
    for (let i = items.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [items[i], items[j]] = [items[j]!, items[i]!];
    }
    queue.value = items;
    goTo(0);
};

const onTimeUpdate = () => {
    const el = videoEl.value;
    const marker = current.value;
    if (!el || !marker || !started.value) return;
    const end = clipEnd(marker);
    const length = Math.max(end - marker.timestamp, 1);
    clipProgress.value = Math.min(1, Math.max(0, (el.currentTime - marker.timestamp) / length));
    if (el.currentTime >= end) nextClip();
};

// A clip that cannot be played is skipped
const onError = () => {
    if (started.value && queue.value.length > 1) setTimeout(nextClip, 500);
};

onMounted(() => {
    // Show the first frame of the first clip before starting
    if (videoEl.value && current.value) {
        videoEl.value.setAttribute('src', streamUrl(current.value));
        videoEl.value.addEventListener(
            'loadedmetadata',
            () => {
                if (videoEl.value && current.value && !started.value) {
                    videoEl.value.currentTime = current.value.timestamp;
                }
            },
            { once: true },
        );
    }
});
</script>

<template>
    <div
        v-if="queue.length === 0"
        class="border-border text-dim flex h-32 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        No markers to compile
    </div>
    <div v-else class="border-border overflow-hidden rounded-lg border bg-black/40">
        <div class="grid gap-0 lg:grid-cols-[1fr_18rem]">
            <!-- Player -->
            <div class="relative bg-black">
                <video
                    ref="videoEl"
                    class="aspect-video w-full bg-black"
                    playsinline
                    preload="metadata"
                    @timeupdate="onTimeUpdate"
                    @error="onError"
                    @play="playing = true"
                    @pause="playing = false"
                />
                <video ref="preloadEl" class="hidden" muted preload="metadata" />
                <button
                    v-if="!started"
                    class="absolute inset-0 flex items-center justify-center bg-black/40
                        transition-colors hover:bg-black/20"
                    aria-label="Play compilation"
                    @click="togglePlay"
                >
                    <span
                        class="bg-lava/90 flex h-14 w-14 items-center justify-center rounded-full
                            shadow-lg"
                    >
                        <Icon name="heroicons:play-solid" size="26" class="text-white" />
                    </span>
                </button>
                <!-- Clip progress -->
                <div class="absolute inset-x-0 bottom-0 h-1 bg-white/10">
                    <div
                        class="bg-lava h-full transition-[width] duration-200"
                        :style="{ width: `${clipProgress * 100}%` }"
                    />
                </div>
            </div>

            <!-- Controls and queue -->
            <div class="border-border flex flex-col border-t lg:border-t-0 lg:border-l">
                <div class="border-border border-b p-3">
                    <div v-if="current" class="min-w-0">
                        <div class="flex items-center gap-1.5">
                            <span
                                class="h-2 w-2 shrink-0 rounded-full"
                                :style="{ backgroundColor: current.color || '#FF4D4D' }"
                            />
                            <span class="truncate text-xs font-semibold text-white">
                                {{ current.label || 'No label' }}
                            </span>
                            <span class="text-dim ml-auto shrink-0 font-mono text-[10px]">
                                {{ index + 1 }}/{{ queue.length }}
                            </span>
                        </div>
                        <NuxtLink
                            :to="markerWatchUrl(current)"
                            class="text-dim hover:text-lava mt-0.5 block truncate text-[11px]
                                transition-colors"
                            :title="current.scene_title"
                        >
                            {{ current.scene_title }} ·
                            {{ formatMarkerRange(current.timestamp, current.end_timestamp) }}
                        </NuxtLink>
                    </div>
                    <div class="mt-2.5 flex items-center gap-1.5">
                        <button
                            class="border-border text-dim flex h-8 w-8 items-center justify-center
                                rounded-md border transition-colors hover:text-white"
                            aria-label="Previous clip"
                            @click="previousClip"
                        >
                            <Icon name="heroicons:backward" size="14" />
                        </button>
                        <button
                            class="bg-lava hover:bg-lava/80 flex h-8 w-10 items-center
                                justify-center rounded-md text-white transition-colors"
                            :aria-label="playing ? 'Pause' : 'Play'"
                            @click="togglePlay"
                        >
                            <Icon
                                :name="playing ? 'heroicons:pause-solid' : 'heroicons:play-solid'"
                                size="15"
                            />
                        </button>
                        <button
                            class="border-border text-dim flex h-8 w-8 items-center justify-center
                                rounded-md border transition-colors hover:text-white"
                            aria-label="Next clip"
                            @click="nextClip"
                        >
                            <Icon name="heroicons:forward" size="14" />
                        </button>
                        <button
                            class="border-border text-dim hover:text-lava ml-auto flex h-8
                                items-center gap-1 rounded-md border px-2 text-[11px]
                                transition-colors"
                            title="Shuffle the order"
                            @click="shuffle"
                        >
                            <Icon name="heroicons:arrows-right-left" size="13" />
                            Shuffle
                        </button>
                        <button
                            class="border-border text-dim hover:text-lava flex h-8 w-8 items-center
                                justify-center rounded-md border transition-colors"
                            title="Load other markers"
                            @click="emit('refresh')"
                        >
                            <Icon name="heroicons:arrow-path" size="13" />
                        </button>
                    </div>
                </div>
                <ol class="max-h-56 flex-1 overflow-y-auto p-1.5 lg:max-h-none">
                    <li v-for="(marker, i) in queue" :key="`${marker.id}-${i}`">
                        <button
                            class="flex w-full items-center gap-2 rounded px-2 py-1 text-left
                                text-[11px] transition-colors"
                            :class="
                                i === index
                                    ? 'bg-lava/10 text-white'
                                    : 'text-dim hover:bg-white/5 hover:text-white'
                            "
                            @click="
                                started = true;
                                playing = true;
                                goTo(i);
                            "
                        >
                            <span class="w-5 shrink-0 font-mono text-[10px] opacity-60">
                                {{ i + 1 }}
                            </span>
                            <span class="min-w-0 flex-1 truncate">
                                {{ marker.label || 'No label' }} · {{ marker.scene_title }}
                            </span>
                            <span class="shrink-0 font-mono text-[10px] opacity-60">
                                {{ formatMarkerRange(marker.timestamp, marker.end_timestamp) }}
                            </span>
                        </button>
                    </li>
                </ol>
            </div>
        </div>
    </div>
</template>
