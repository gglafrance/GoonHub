<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// saved_search_tiles widget: your saved searches as tiles with their result count.
interface SavedSearchTile {
    uuid: string;
    name: string;
    count: number | null;
    cover_scene_id?: number;
}

interface SavedSearchTilesPayload {
    items: SavedSearchTile[];
    total: number;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<SavedSearchTilesPayload>;
}>();

defineEmits<{ refresh: [] }>();

const items = computed(() => props.data.widget?.items ?? []);
const hidden = computed(() => (props.data.widget?.total ?? 0) - items.value.length);
const number = new Intl.NumberFormat();
</script>

<template>
    <div
        v-if="items.length === 0"
        class="border-border text-dim flex h-28 flex-col items-center justify-center gap-1
            rounded-lg border border-dashed text-xs"
    >
        <span>No saved searches yet.</span>
        <NuxtLink to="/search" class="text-lava hover:underline">Create one from Search</NuxtLink>
    </div>
    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6">
        <NuxtLink
            v-for="tile in items"
            :key="tile.uuid"
            :to="`/search?saved=${tile.uuid}`"
            class="border-border hover:border-lava/40 group relative aspect-video overflow-hidden
                rounded-lg border bg-black/40 transition-all"
        >
            <img
                v-if="tile.cover_scene_id"
                :src="`/thumbnails/${tile.cover_scene_id}`"
                :alt="tile.name"
                loading="lazy"
                class="absolute inset-0 h-full w-full object-cover opacity-50 transition-all
                    duration-300 group-hover:scale-105 group-hover:opacity-70"
            />
            <div
                class="absolute inset-0 bg-linear-to-t from-black/90 via-black/40 to-transparent"
            />
            <div class="absolute inset-x-0 bottom-0 p-2.5">
                <div class="flex items-center gap-1.5">
                    <Icon name="heroicons:magnifying-glass" size="12" class="text-lava shrink-0" />
                    <span class="truncate text-xs font-medium text-white" :title="tile.name">
                        {{ tile.name }}
                    </span>
                </div>
                <div class="text-dim mt-0.5 font-mono text-[10px]">
                    {{
                        tile.count === null ? 'unavailable' : `${number.format(tile.count)} scenes`
                    }}
                </div>
            </div>
        </NuxtLink>
        <NuxtLink
            v-if="hidden > 0"
            to="/search"
            class="border-border text-dim hover:border-lava/40 hover:text-lava flex aspect-video
                items-center justify-center rounded-lg border border-dashed text-xs transition-all"
        >
            +{{ hidden }} more
        </NuxtLink>
    </div>
</template>
