<script setup lang="ts">
import type { ShortItem } from '~/types/shorts';

// Seconds of playback before a short counts as a view.
const VIEW_AFTER_SECONDS = 5;

const props = defineProps<{
    item: ShortItem;
    active: boolean;
    // Active slide or a neighbour: only these get a src, so the feed never
    // holds more than three streams.
    loaded: boolean;
    preloadNext: boolean;
    muted: boolean;
    liked: boolean;
    rating: number;
    jizzCount: number;
    maximized: boolean;
    infoOpen: boolean;
}>();

const emit = defineEmits<{
    'update:muted': [muted: boolean];
    toggleMaximize: [];
    toggleInfo: [];
}>();

const { recordWatch } = useApiScenes();

const video = ref<HTMLVideoElement | null>(null);
const currentTime = ref(0);
const duration = ref(0);
const paused = ref(true);
// The browser blocked sound: playing muted until the viewer taps.
const needsTap = ref(false);

let playedSeconds = 0;
let lastTime = 0;
let viewRecorded = false;

const streamUrl = computed(() => `/api/v1/scenes/${props.item.id}/stream`);
const posterUrl = computed(() => {
    if (!props.item.thumbnail_path) return undefined;
    const base = `/thumbnails/${props.item.id}?size=lg`;
    const v = props.item.updated_at ? new Date(props.item.updated_at).getTime() : 0;
    return v ? `${base}&v=${v}` : base;
});

// Try with sound first; if the browser refuses, play muted with a badge.
const play = async () => {
    const v = video.value;
    if (!v) return;
    v.muted = props.muted;
    try {
        await v.play();
        needsTap.value = false;
    } catch {
        if (!props.active) return;
        v.muted = true;
        needsTap.value = !props.muted;
        try {
            await v.play();
        } catch {
            // Autoplay blocked entirely; the viewer can tap to play
        }
    }
};

const pause = () => {
    video.value?.pause();
};

watch(
    () => props.active,
    (active) => {
        if (active) {
            nextTick(play);
        } else {
            pause();
            if (video.value) video.value.currentTime = 0;
        }
    },
);

watch(
    () => props.muted,
    (m) => {
        if (video.value) video.value.muted = m;
        if (!m) needsTap.value = false;
    },
);

const onLoaded = () => {
    duration.value = video.value?.duration ?? 0;
    if (props.active) play();
};

const onTimeUpdate = () => {
    const v = video.value;
    if (!v) return;
    currentTime.value = v.currentTime;
    const delta = v.currentTime - lastTime;
    // Ignore seeks and the jump back when the clip loops
    if (delta > 0 && delta < 1.5) playedSeconds += delta;
    lastTime = v.currentTime;
    if (!viewRecorded && playedSeconds >= VIEW_AFTER_SECONDS) {
        viewRecorded = true;
        recordWatch(
            props.item.id,
            Math.round(duration.value),
            Math.floor(v.currentTime),
            false,
        ).catch(() => {});
    }
};

const togglePlay = () => {
    const v = video.value;
    if (!v) return;
    if (needsTap.value) {
        emit('update:muted', false);
        v.muted = false;
        needsTap.value = false;
        return;
    }
    if (v.paused) play();
    else pause();
};

const seek = (t: number) => {
    if (video.value) video.value.currentTime = t;
};

// Keyboard scrubbing, clamped to the clip
const seekBy = (delta: number) => {
    const v = video.value;
    if (!v || !Number.isFinite(v.duration)) return;
    v.currentTime = Math.min(v.duration, Math.max(0, v.currentTime + delta));
    lastTime = v.currentTime;
};

// A click on the video plays/pauses once it is clear no second click follows;
// a double click toggles maximize instead.
const DOUBLE_CLICK_MS = 250;
let clickTimer: ReturnType<typeof setTimeout> | undefined;

const onVideoClick = () => {
    clearTimeout(clickTimer);
    clickTimer = setTimeout(togglePlay, DOUBLE_CLICK_MS);
};

const onVideoDoubleClick = () => {
    clearTimeout(clickTimer);
    emit('toggleMaximize');
};

onBeforeUnmount(() => clearTimeout(clickTimer));

defineExpose({ togglePlay, seekBy });

const clipRange = computed(() => {
    const { source_start: s, source_end: e } = props.item;
    return s !== null && e !== null;
});
</script>

<template>
    <div class="relative flex h-full w-full items-center justify-center bg-black">
        <video
            ref="video"
            :src="loaded ? streamUrl : undefined"
            :poster="posterUrl"
            :preload="active || preloadNext ? 'auto' : 'metadata'"
            loop
            playsinline
            class="h-full w-full object-contain"
            @loadedmetadata="onLoaded"
            @timeupdate="onTimeUpdate"
            @play="paused = false"
            @pause="paused = true"
            @click="onVideoClick"
            @dblclick="onVideoDoubleClick"
        />

        <!-- Paused indicator -->
        <div
            v-if="active && paused && !needsTap"
            class="pointer-events-none absolute inset-0 flex items-center justify-center"
        >
            <div
                class="flex h-16 w-16 items-center justify-center rounded-full bg-black/40
                    backdrop-blur-sm"
            >
                <Icon name="heroicons:play-solid" size="30" class="text-white/90" />
            </div>
        </div>

        <!-- Sound was blocked -->
        <button
            v-if="active && needsTap"
            class="border-lava/40 absolute top-4 left-1/2 flex -translate-x-1/2 items-center gap-1.5
                rounded-full border bg-black/60 px-3 py-1.5 text-xs font-medium text-white
                backdrop-blur-md"
            @click="togglePlay"
        >
            <Icon name="heroicons:speaker-x-mark" size="14" class="text-lava" />
            Tap for sound
        </button>

        <!-- Title and origin -->
        <div
            class="pointer-events-none absolute right-16 bottom-6 left-0 bg-linear-to-t
                from-black/70 to-transparent px-4 pt-10 pb-2"
        >
            <NuxtLink
                :to="`/watch/${item.id}`"
                class="pointer-events-auto line-clamp-2 text-sm font-semibold text-white drop-shadow
                    hover:underline"
            >
                {{ item.title }}
            </NuxtLink>
            <div class="mt-1 flex items-center gap-2 font-mono text-[10px] text-white/60">
                <span>{{ Math.round(item.duration) }}s</span>
                <span v-if="clipRange" class="flex items-center gap-1">
                    <Icon name="heroicons:scissors" size="10" />
                    Created short
                </span>
                <span>{{ item.view_count }} views</span>
            </div>
        </div>

        <ShortsActionRail
            :scene-id="item.id"
            :liked="liked"
            :rating="rating"
            :jizz-count="jizzCount"
            :muted="muted"
            :maximized="maximized"
            :info-open="infoOpen"
            class="absolute right-2 bottom-10"
            @toggle-mute="emit('update:muted', !muted)"
            @toggle-maximize="emit('toggleMaximize')"
            @toggle-info="emit('toggleInfo')"
        />

        <ShortsSeekBar
            class="absolute right-0 bottom-0 left-0"
            :current-time="currentTime"
            :duration="duration"
            @seek="seek"
        />
    </div>
</template>
