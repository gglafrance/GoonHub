<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import type { BrowseTagsPayload, WidgetTile } from '~/types/homepageMarkerWidgets';

// browse_tags widget: tiles of marker labels or scene tags with a cover image.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData;
}>();

const payload = computed(() => props.data.widget as BrowseTagsPayload | undefined);
const tiles = computed(() => payload.value?.tiles ?? []);
const isTags = computed(() => payload.value?.source === 'tags');

const tileLink = (tile: WidgetTile) =>
    isTags.value
        ? { path: '/search', query: { tags: tile.key } }
        : `/markers/${encodeURIComponent(tile.key)}`;

const coverUrl = (tile: WidgetTile) => {
    if (tile.cover_marker_id) {
        return markerThumbnailUrl(tile.cover_marker_id, tile.cover_marker_version);
    }
    if (tile.cover_scene_id) return `/thumbnails/${tile.cover_scene_id}?size=sm`;
    return null;
};
</script>

<template>
    <div
        v-if="tiles.length === 0"
        class="border-border text-dim flex h-32 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        {{ isTags ? 'No tags yet' : 'No marker labels yet' }}
    </div>
    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
        <NuxtLink
            v-for="tile in tiles"
            :key="tile.key"
            :to="tileLink(tile)"
            class="group border-border hover:border-lava/30 relative block aspect-video
                overflow-hidden rounded-lg border bg-black/40 transition-all"
        >
            <img
                v-if="coverUrl(tile)"
                :src="coverUrl(tile)!"
                :alt="tile.label"
                loading="lazy"
                class="h-full w-full object-cover opacity-70 transition-all duration-300
                    group-hover:scale-105 group-hover:opacity-90"
            />
            <div
                class="absolute inset-0 bg-linear-to-t from-black/85 via-black/30 to-transparent"
            />
            <div class="absolute inset-x-0 bottom-0 flex items-end justify-between gap-2 p-2.5">
                <div class="flex min-w-0 items-center gap-1.5">
                    <span
                        v-if="tile.color"
                        class="h-2 w-2 shrink-0 rounded-full"
                        :style="{ backgroundColor: tile.color }"
                    />
                    <Icon v-else name="heroicons:bookmark" size="12" class="text-lava shrink-0" />
                    <span class="truncate text-xs font-semibold text-white">{{ tile.label }}</span>
                </div>
                <span
                    class="shrink-0 rounded-full bg-black/60 px-1.5 py-0.5 font-mono text-[10px]
                        text-white/80"
                >
                    {{ tile.count }}
                </span>
            </div>
        </NuxtLink>
    </div>
</template>
