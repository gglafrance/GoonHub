<script setup lang="ts">
import type { MarkerLabelGroup } from '~/types/marker';

const props = defineProps<{
    group: MarkerLabelGroup;
}>();

const thumbnailUrl = computed(() => `/marker-thumbnails/${props.group.thumbnail_marker_id}`);
</script>

<template>
    <NuxtLink
        :to="`/markers/${encodeURIComponent(group.label)}`"
        class="group relative block max-w-[320px] overflow-hidden rounded-lg"
    >
        <!-- Thumbnail -->
        <div class="relative aspect-video w-full overflow-hidden bg-black/40">
            <img
                :src="thumbnailUrl"
                :alt="group.label"
                class="h-full w-full object-cover transition-transform duration-300
                    group-hover:scale-105"
                loading="lazy"
            />

            <!-- Gradient overlay -->
            <div
                class="pointer-events-none absolute inset-0 bg-linear-to-t from-black/80
                    via-black/20 to-transparent"
            />

            <!-- Count badge -->
            <div
                class="absolute top-2 right-2 rounded-md bg-black/70 px-1.5 py-0.5 text-[10px]
                    font-semibold text-white/90 backdrop-blur-sm"
            >
                {{ group.count }} {{ group.count === 1 ? 'marker' : 'markers' }}
            </div>
        </div>

        <!-- Label text -->
        <div class="border-border bg-surface border-t px-2.5 py-2">
            <p class="truncate text-sm font-medium text-white group-hover:text-white">
                {{ group.label }}
            </p>
        </div>
    </NuxtLink>
</template>
