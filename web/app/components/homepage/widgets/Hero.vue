<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// hero widget: a large banner featuring one scene at a time, playing the original video (muted,
// full quality, letterboxed over a blurred backdrop), with its actors and tags, Play / Next, and
// an optional automatic rotation. Falls back to the generated preview, then the thumbnail, when
// the browser cannot play the original file.
interface HeroDetails {
    actors: string[];
    tags: string[];
    liked: boolean;
    jizz_count: number;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{
        source: string;
        rotate_seconds: number;
        details: Record<string, HeroDetails>;
    }>;
}>();

const { formatDuration } = useFormatter();

const index = ref(0);
const paused = ref(false);
let timer: ReturnType<typeof setInterval> | null = null;

const scenes = computed(() => props.data.scenes);
const scene = computed(() => scenes.value[index.value % Math.max(scenes.value.length, 1)]);
const details = computed<HeroDetails | null>(() =>
    scene.value ? (props.data.widget?.details?.[String(scene.value.id)] ?? null) : null,
);
const version = computed(() =>
    scene.value?.updated_at ? new Date(scene.value.updated_at).getTime() : '',
);

// Start the original video away from the intro
const START_FRACTION = 0.2;
// Scenes whose original file failed to play in this browser (fall back to the preview)
const streamFailed = ref<Set<number>>(new Set());

const useStream = computed(() => !!scene.value && !streamFailed.value.has(scene.value.id));

const onStreamLoaded = (event: Event) => {
    const el = event.target as HTMLVideoElement;
    if (el.duration && isFinite(el.duration)) {
        el.currentTime = el.duration * START_FRACTION;
    }
    el.play().catch(() => {});
};

const onStreamError = () => {
    if (!scene.value) return;
    streamFailed.value = new Set([...streamFailed.value, scene.value.id]);
};

const go = (offset: number) => {
    if (scenes.value.length === 0) return;
    index.value = (index.value + offset + scenes.value.length) % scenes.value.length;
};

const restartTimer = () => {
    if (timer) clearInterval(timer);
    timer = null;
    const seconds = props.data.widget?.rotate_seconds ?? 0;
    if (seconds > 0 && scenes.value.length > 1) {
        timer = setInterval(() => {
            if (!paused.value) go(1);
        }, seconds * 1000);
    }
};

watch(scenes, () => {
    index.value = 0;
    restartTimer();
});

onMounted(restartTimer);
onBeforeUnmount(() => {
    if (timer) clearInterval(timer);
});
</script>

<template>
    <div
        v-if="!scene"
        class="border-border text-dim flex h-48 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        No scene to feature
    </div>
    <div
        v-else
        class="border-border bg-void relative aspect-video max-h-[60vh] w-full overflow-hidden
            rounded-xl border sm:aspect-21/9"
        @mouseenter="paused = true"
        @mouseleave="paused = false"
    >
        <Transition
            mode="out-in"
            enter-active-class="transition-opacity duration-500"
            enter-from-class="opacity-0"
            leave-active-class="transition-opacity duration-300"
            leave-to-class="opacity-0"
        >
            <div :key="scene.id" class="absolute inset-0">
                <!-- Blurred backdrop filling the banner around the letterboxed video -->
                <img
                    :src="`/thumbnails/${scene.id}?size=lg&v=${version}`"
                    alt=""
                    aria-hidden="true"
                    class="absolute inset-0 h-full w-full scale-110 object-cover opacity-60
                        blur-2xl"
                />
                <video
                    v-if="useStream"
                    :src="`/api/v1/scenes/${scene.id}/stream`"
                    :poster="`/thumbnails/${scene.id}?size=lg&v=${version}`"
                    muted
                    loop
                    playsinline
                    preload="metadata"
                    class="relative h-full w-full object-contain"
                    @loadedmetadata="onStreamLoaded"
                    @error="onStreamError"
                />
                <video
                    v-else-if="scene.preview_video_path"
                    :src="`/scene-previews/${scene.id}?v=${version}`"
                    :poster="`/thumbnails/${scene.id}?size=lg&v=${version}`"
                    autoplay
                    muted
                    loop
                    playsinline
                    class="relative h-full w-full object-contain"
                />
                <img
                    v-else
                    :src="`/thumbnails/${scene.id}?size=lg&v=${version}`"
                    :alt="scene.title"
                    class="relative h-full w-full object-contain"
                />
            </div>
        </Transition>

        <!-- Overlay -->
        <div
            class="absolute inset-0 bg-linear-to-t from-black/90 via-black/30 to-transparent
                sm:bg-linear-to-r sm:from-black/85 sm:via-black/40"
        />
        <div class="absolute inset-x-0 bottom-0 p-4 sm:top-0 sm:flex sm:w-1/2 sm:items-end sm:p-8">
            <div class="space-y-2.5">
                <div
                    class="text-lava flex items-center gap-1.5 text-[10px] font-semibold uppercase"
                >
                    <Icon name="heroicons:star-solid" size="11" />
                    Spotlight
                </div>
                <h3 class="line-clamp-2 text-lg font-bold text-white sm:text-2xl">
                    {{ scene.title }}
                </h3>
                <div class="text-dim flex flex-wrap items-center gap-2 font-mono text-[11px]">
                    <span>{{ formatDuration(scene.duration) }}</span>
                    <span v-if="details?.liked" class="flex items-center gap-0.5 text-pink-400">
                        <Icon name="heroicons:heart-solid" size="11" />
                    </span>
                    <span v-if="details?.jizz_count" class="text-lava">
                        x{{ details.jizz_count }}
                    </span>
                </div>
                <div v-if="details?.actors.length" class="truncate text-xs text-white/80">
                    {{ details.actors.slice(0, 4).join(' · ') }}
                </div>
                <div v-if="details?.tags.length" class="flex flex-wrap gap-1">
                    <span
                        v-for="tag in details.tags.slice(0, 6)"
                        :key="tag"
                        class="rounded border border-white/15 bg-black/30 px-1.5 py-px text-[10px]
                            text-white/80 backdrop-blur-sm"
                    >
                        {{ tag }}
                    </span>
                </div>
                <div class="flex items-center gap-2 pt-1">
                    <NuxtLink
                        :to="`/watch/${scene.id}`"
                        class="bg-lava hover:bg-lava/80 flex items-center gap-1.5 rounded-lg px-4
                            py-2 text-xs font-semibold text-white transition-colors"
                    >
                        <Icon name="heroicons:play-solid" size="14" />
                        Play
                    </NuxtLink>
                    <button
                        v-if="scenes.length > 1"
                        class="flex items-center gap-1.5 rounded-lg border border-white/20
                            bg-black/30 px-3 py-2 text-xs text-white backdrop-blur-sm
                            transition-colors hover:bg-white/10"
                        @click="go(1)"
                    >
                        Next
                        <Icon name="heroicons:chevron-right" size="14" />
                    </button>
                </div>
            </div>
        </div>

        <!-- Pool position -->
        <div v-if="scenes.length > 1" class="absolute right-4 bottom-4 flex gap-1">
            <button
                v-for="(s, i) in scenes"
                :key="s.id"
                class="h-1.5 rounded-full transition-all"
                :class="i === index ? 'bg-lava w-5' : 'w-1.5 bg-white/30 hover:bg-white/60'"
                :aria-label="`Show ${s.title}`"
                @click="index = i"
            />
        </div>
    </div>
</template>
