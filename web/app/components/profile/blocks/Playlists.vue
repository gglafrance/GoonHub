<script setup lang="ts">
import type { ProfileBlock, ProfilePlaylistsPayload } from '~/types/profile';

// playlists: the owner's public playlists (profile block). The cover is the thumbnail of the
// first scene, sent only to viewers with scenes:view.
const props = defineProps<{ block: ProfileBlock<ProfilePlaylistsPayload>; owner: boolean }>();
const items = computed(() => props.block.widget?.items ?? []);
</script>

<template>
    <div class="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3">
        <NuxtLink
            v-for="p in items"
            :key="p.uuid"
            :to="`/playlists/${p.uuid}`"
            class="group border-border hover:border-border-hover block overflow-hidden rounded-lg
                border bg-white/2 transition-all"
        >
            <div class="relative aspect-video w-full overflow-hidden bg-black/40">
                <img
                    v-if="p.cover_scene_id"
                    :src="`/thumbnails/${p.cover_scene_id}?size=sm`"
                    :alt="p.name"
                    loading="lazy"
                    class="h-full w-full object-cover transition-transform duration-300
                        group-hover:scale-105"
                />
                <div v-else class="flex h-full w-full items-center justify-center">
                    <Icon name="heroicons:queue-list" size="22" class="text-dim/40" />
                </div>
                <div
                    class="absolute right-1.5 bottom-1.5 rounded bg-black/70 px-1.5 py-0.5 font-mono
                        text-[10px] text-white backdrop-blur-sm"
                >
                    {{ p.scene_count }}
                </div>
            </div>
            <div class="truncate px-2 py-1.5 text-[11px] font-medium text-white">{{ p.name }}</div>
        </NuxtLink>
    </div>
</template>
