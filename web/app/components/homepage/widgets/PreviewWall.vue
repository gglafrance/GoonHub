<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import type { SceneListItem } from '~/types/scene';

// preview_wall widget: a mosaic of previews all playing muted while the wall is on screen;
// hovering a tile unmutes it (hover audio volume), clicking opens the scene.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ source: string; columns: number }>;
}>();

const { enabled: hoverAudio, volume } = useHoverAudio();

const root = ref<HTMLElement | null>(null);
const onScreen = ref(false);
const hoveredId = ref<number | null>(null);
const videos = new Map<number, HTMLVideoElement>();
let observer: IntersectionObserver | null = null;

const columns = computed(() => Math.min(6, Math.max(3, props.data.widget?.columns ?? 4)));
const gridStyle = computed(() => ({
    gridTemplateColumns: `repeat(${columns.value}, minmax(0, 1fr))`,
}));

const version = (scene: SceneListItem) =>
    scene.updated_at ? new Date(scene.updated_at).getTime() : '';

const setVideo = (id: number, el: Element | null) => {
    if (el) videos.set(id, el as HTMLVideoElement);
    else videos.delete(id);
};

const sync = () => {
    videos.forEach((el, id) => {
        const audible = id === hoveredId.value && hoverAudio.value && volume.value > 0;
        el.volume = volume.value / 100;
        el.muted = !audible;
        if (onScreen.value) el.play().catch(() => {});
        else el.pause();
    });
};

watch([onScreen, hoveredId, hoverAudio, volume], sync);
watch(
    () => props.data.scenes,
    async () => {
        await nextTick();
        sync();
    },
);

onMounted(() => {
    if (!root.value) return;
    observer = new IntersectionObserver(
        ([entry]) => {
            onScreen.value = !!entry?.isIntersecting;
        },
        { threshold: 0.15 },
    );
    observer.observe(root.value);
});
onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
    <div ref="root">
        <div
            v-if="data.scenes.length === 0"
            class="border-border text-dim flex h-40 items-center justify-center rounded-lg border
                border-dashed text-xs"
        >
            No scenes to show
        </div>
        <div v-else class="grid gap-1.5" :style="gridStyle">
            <NuxtLink
                v-for="scene in data.scenes"
                :key="scene.id"
                :to="`/watch/${scene.id}`"
                class="group bg-void relative aspect-video overflow-hidden rounded-md"
                :title="scene.title"
                @mouseenter="hoveredId = scene.id"
                @mouseleave="hoveredId = null"
            >
                <video
                    v-if="scene.preview_video_path"
                    :ref="(el) => setVideo(scene.id, el as Element | null)"
                    :src="`/scene-previews/${scene.id}?v=${version(scene)}`"
                    :poster="`/thumbnails/${scene.id}?v=${version(scene)}`"
                    muted
                    loop
                    playsinline
                    preload="none"
                    class="h-full w-full object-cover"
                />
                <img
                    v-else
                    :src="`/thumbnails/${scene.id}?v=${version(scene)}`"
                    :alt="scene.title"
                    loading="lazy"
                    class="h-full w-full object-cover"
                />
                <div
                    class="ring-lava/0 group-hover:ring-lava/60 pointer-events-none absolute inset-0
                        rounded-md ring-1 transition-all ring-inset"
                />
                <div
                    class="pointer-events-none absolute inset-x-0 bottom-0 truncate bg-linear-to-t
                        from-black/80 to-transparent px-2 pt-4 pb-1 text-[10px] text-white opacity-0
                        transition-opacity group-hover:opacity-100"
                >
                    {{ scene.title }}
                </div>
            </NuxtLink>
        </div>
    </div>
</template>
