<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

// A short clip of the shorts feed: plays (muted) only while active, hover unmutes.
const props = defineProps<{
    scene: SceneListItem;
    active: boolean;
}>();

const { formatDuration } = useFormatter();
const { enabled: hoverAudio, volume } = useHoverAudio();

const video = ref<HTMLVideoElement | null>(null);
const hovering = ref(false);

const version = computed(() =>
    props.scene.updated_at ? new Date(props.scene.updated_at).getTime() : '',
);
const thumbnailUrl = computed(() => `/thumbnails/${props.scene.id}?size=lg&v=${version.value}`);
// Short clips play the file itself; the generated preview is used when there is one
const videoUrl = computed(() =>
    props.scene.preview_video_path
        ? `/scene-previews/${props.scene.id}?v=${version.value}`
        : `/api/v1/scenes/${props.scene.id}/stream`,
);

const applyAudio = () => {
    const el = video.value;
    if (!el) return;
    el.volume = volume.value / 100;
    el.muted = !(hovering.value && hoverAudio.value && volume.value > 0);
};

watch(
    () => [props.active, video.value] as const,
    ([active, el]) => {
        if (!el) return;
        if (active) {
            applyAudio();
            el.play().catch(() => {
                el.muted = true;
                el.play().catch(() => {});
            });
        } else {
            el.pause();
        }
    },
);

watch([hovering, hoverAudio, volume], applyAudio);
</script>

<template>
    <NuxtLink
        :to="`/watch/${scene.id}`"
        class="group border-border bg-void relative block aspect-3/4 w-44 shrink-0 snap-start
            overflow-hidden rounded-lg border transition-all hover:border-white/20 sm:w-52"
        @mouseenter="hovering = true"
        @mouseleave="hovering = false"
    >
        <img
            :src="thumbnailUrl"
            :alt="scene.title"
            loading="lazy"
            class="absolute inset-0 h-full w-full object-cover"
        />
        <video
            v-if="active"
            ref="video"
            :src="videoUrl"
            muted
            loop
            playsinline
            preload="metadata"
            class="absolute inset-0 h-full w-full object-cover"
        />
        <div
            class="pointer-events-none absolute inset-x-0 bottom-0 bg-linear-to-t from-black/90
                via-black/40 to-transparent p-2.5 pt-8"
        >
            <div class="line-clamp-2 text-[11px] font-medium text-white">{{ scene.title }}</div>
            <div class="text-dim mt-0.5 font-mono text-[10px]">
                {{ formatDuration(scene.duration) }}
            </div>
        </div>
        <div
            v-if="active && hovering && hoverAudio && volume > 0"
            class="absolute top-2 right-2 rounded bg-black/60 p-1 text-white backdrop-blur-sm"
        >
            <Icon name="heroicons:speaker-wave" size="12" />
        </div>
    </NuxtLink>
</template>
