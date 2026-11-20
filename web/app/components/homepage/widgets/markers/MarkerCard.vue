<script setup lang="ts">
import type { WidgetMarker } from '~/types/homepageMarkerWidgets';

// Marker preview card of the marker widgets: animated preview when available (played only
// while visible), static thumbnail otherwise. Clicking opens the scene at the marker (a range
// marker loops on the watch page).
const props = defineProps<{
    marker: WidgetMarker;
}>();

const { formatMarkerRange } = useFormatter();

const videoEl = ref<HTMLVideoElement | null>(null);
let observer: IntersectionObserver | null = null;

const to = computed(() => markerWatchUrl(props.marker));

onMounted(() => {
    if (!videoEl.value) return;
    observer = new IntersectionObserver(
        (entries) => {
            for (const entry of entries) {
                const video = entry.target as HTMLVideoElement;
                if (entry.isIntersecting) {
                    video.play().catch(() => {});
                } else {
                    video.pause();
                }
            }
        },
        { threshold: 0.3 },
    );
    observer.observe(videoEl.value);
});

onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
    <NuxtLink
        :to="to"
        class="group border-border hover:border-border-hover block overflow-hidden rounded-lg border
            bg-white/2 transition-all"
    >
        <div class="relative aspect-video w-full overflow-hidden bg-black/40">
            <MarkersTagsBadge :tags="marker.tags" :label="marker.label" />
            <video
                v-if="marker.has_animated"
                ref="videoEl"
                :src="`/marker-thumbnails/${marker.id}/animated`"
                muted
                loop
                playsinline
                preload="none"
                class="h-full w-full object-contain transition-transform duration-300
                    group-hover:scale-105"
            />
            <img
                v-else-if="marker.has_thumbnail"
                :src="`/marker-thumbnails/${marker.id}`"
                :alt="marker.label || marker.scene_title"
                loading="lazy"
                class="h-full w-full object-cover transition-transform duration-300
                    group-hover:scale-105"
            />
            <div v-else class="flex h-full w-full items-center justify-center">
                <Icon name="heroicons:bookmark" size="22" class="text-dim/40" />
            </div>

            <div
                class="absolute right-1.5 bottom-1.5 rounded bg-black/70 px-1.5 py-0.5 font-mono
                    text-[10px] text-white backdrop-blur-sm"
            >
                {{ formatMarkerRange(marker.timestamp, marker.end_timestamp) }}
            </div>
            <div
                class="bg-lava/20 absolute inset-0 flex items-center justify-center opacity-0
                    transition-opacity group-hover:opacity-100"
            >
                <Icon name="heroicons:play-solid" size="26" class="text-white" />
            </div>
        </div>
        <div class="px-2 py-1.5">
            <div class="flex items-center gap-1.5">
                <span
                    class="h-1.5 w-1.5 shrink-0 rounded-full"
                    :style="{ backgroundColor: marker.color || '#FF4D4D' }"
                />
                <span class="truncate text-[11px] font-medium text-white">
                    {{ marker.label || 'No label' }}
                </span>
            </div>
            <div class="text-dim truncate text-[10px]" :title="marker.scene_title">
                {{ marker.scene_title }}
            </div>
        </div>
    </NuxtLink>
</template>
