<script setup lang="ts">
import type { Scene } from '~/types/scene';

// Large media area of the triage page: the animated preview (or the thumbnail) until the
// user plays the scene, then a plain video player streaming the scene.
const props = defineProps<{
    scene: Scene | null;
    loading?: boolean;
}>();

const playing = ref(false);
const video = ref<HTMLVideoElement | null>(null);

const version = computed(() =>
    props.scene?.updated_at ? new Date(props.scene.updated_at).getTime() : '',
);

const thumbnailUrl = computed(() => {
    if (!props.scene?.thumbnail_path) return '';
    const base = `/thumbnails/${props.scene.id}?size=lg`;
    return version.value ? `${base}&v=${version.value}` : base;
});

const previewUrl = computed(() => {
    if (!props.scene?.preview_video_path) return '';
    const base = `/scene-previews/${props.scene.id}`;
    return version.value ? `${base}?v=${version.value}` : base;
});

const streamUrl = computed(() => (props.scene ? `/api/v1/scenes/${props.scene.id}/stream` : ''));

// Each scene starts on its preview
watch(
    () => props.scene?.id,
    () => {
        playing.value = false;
    },
);

const play = async () => {
    if (!props.scene) return;
    if (!playing.value) {
        playing.value = true;
        return;
    }
    const el = video.value;
    if (!el) return;
    if (el.paused) await el.play().catch(() => {});
    else el.pause();
};

// Start playing as soon as the player mounts
watch(video, (el) => {
    el?.play().catch(() => {});
});

defineExpose({ togglePlay: play });
</script>

<template>
    <div
        class="border-border relative aspect-video w-full overflow-hidden rounded-xl border
            bg-black"
    >
        <div v-if="loading && !scene" class="absolute inset-0 flex items-center justify-center">
            <LoadingSpinner />
        </div>

        <template v-else-if="scene">
            <video
                v-if="playing"
                ref="video"
                :src="streamUrl"
                :poster="thumbnailUrl || undefined"
                class="h-full w-full bg-black object-contain"
                controls
                playsinline
                tabindex="-1"
            />
            <button
                v-else
                class="group absolute inset-0 block h-full w-full cursor-pointer"
                title="Play (P)"
                tabindex="-1"
                @click="play"
            >
                <video
                    v-if="previewUrl"
                    :key="previewUrl"
                    :src="previewUrl"
                    :poster="thumbnailUrl || undefined"
                    class="h-full w-full object-contain"
                    autoplay
                    muted
                    loop
                    playsinline
                />
                <img
                    v-else-if="thumbnailUrl"
                    :src="thumbnailUrl"
                    :alt="scene.title"
                    class="h-full w-full object-contain"
                />
                <div v-else class="text-dim flex h-full items-center justify-center text-xs">
                    No preview
                </div>
                <div
                    class="absolute inset-0 flex items-center justify-center bg-black/0
                        transition-colors group-hover:bg-black/30"
                >
                    <div
                        class="border-lava/40 bg-void/70 flex h-14 w-14 items-center justify-center
                            rounded-full border opacity-80 backdrop-blur-sm transition-all
                            group-hover:scale-105 group-hover:opacity-100"
                    >
                        <Icon name="heroicons:play-solid" size="24" class="text-lava ml-0.5" />
                    </div>
                </div>
                <span
                    v-if="previewUrl"
                    class="bg-void/70 text-dim absolute top-2 left-2 rounded px-1.5 py-0.5 font-mono
                        text-[10px] tracking-wider uppercase backdrop-blur-sm"
                    >Preview</span
                >
            </button>
        </template>

        <div v-else class="text-dim absolute inset-0 flex items-center justify-center text-xs">
            No scene
        </div>
    </div>
</template>
