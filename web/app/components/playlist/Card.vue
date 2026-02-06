<script setup lang="ts">
import type { PlaylistListItem } from '~/types/playlist';

defineProps<{
    playlist: PlaylistListItem;
    fluid?: boolean;
}>();

const { formatDuration } = useFormatter();
</script>

<template>
    <NuxtLink
        :to="`/playlists/${playlist.uuid}`"
        class="group border-border bg-surface hover:border-border-hover hover:bg-elevated relative
            block overflow-hidden rounded-lg border transition-all duration-200"
        :class="fluid ? 'w-full' : 'w-[280px] sm:w-[320px]'"
    >
        <!-- 2x2 Thumbnail Grid -->
        <div
            class="bg-void relative grid grid-cols-2 grid-rows-2"
            :class="fluid ? 'aspect-video w-full' : 'h-[158px] sm:h-45'"
        >
            <template v-if="playlist.thumbnail_scenes.length > 0">
                <div
                    v-for="(thumb, i) in playlist.thumbnail_scenes.slice(0, 4)"
                    :key="thumb.id"
                    class="relative overflow-hidden"
                    :class="{
                        'col-span-2 row-span-2': playlist.thumbnail_scenes.length === 1,
                        'row-span-2':
                            playlist.thumbnail_scenes.length === 2 ||
                            (playlist.thumbnail_scenes.length === 3 && i === 0),
                    }"
                >
                    <img
                        :src="`/thumbnails/${thumb.id}`"
                        class="h-full w-full object-cover transition-transform duration-300
                            group-hover:scale-[1.03]"
                        :alt="`Scene ${i + 1}`"
                        loading="lazy"
                    />
                </div>
            </template>
            <div v-else class="col-span-2 row-span-2 flex items-center justify-center">
                <Icon name="heroicons:queue-list" size="32" class="text-dim" />
            </div>

            <!-- Scene count badge -->
            <div
                class="bg-void/90 absolute right-1.5 bottom-1.5 z-10 rounded px-1.5 py-0.5 font-mono
                    text-[10px] font-medium text-white backdrop-blur-sm"
            >
                {{ playlist.scene_count }} {{ playlist.scene_count === 1 ? 'scene' : 'scenes' }}
            </div>

            <!-- Visibility badge -->
            <div
                v-if="playlist.visibility !== 'public'"
                class="absolute top-1.5 right-1.5 z-10 rounded px-1.5 py-0.5 text-[9px]
                    font-semibold backdrop-blur-sm"
                :class="
                    playlist.visibility === 'private'
                        ? 'bg-amber-500/90 text-white'
                        : 'bg-blue-500/90 text-white'
                "
            >
                {{ playlist.visibility === 'private' ? 'Private' : 'Unlisted' }}
            </div>

            <!-- Hover overlay -->
            <div
                class="bg-lava/0 group-hover:bg-lava/5 pointer-events-none absolute inset-0 z-10
                    transition-colors duration-200"
            ></div>
        </div>

        <div class="p-3">
            <h3
                class="truncate text-xs font-medium text-white/90 transition-colors
                    group-hover:text-white"
                :title="playlist.name"
            >
                {{ playlist.name }}
            </h3>
            <div class="text-dim mt-1.5 flex items-center justify-between font-mono text-[10px]">
                <span class="flex items-center gap-1.5">
                    <span>{{ formatDuration(Number(playlist.total_duration)) }}</span>
                    <span v-if="playlist.like_count > 0" class="flex items-center gap-0.5">
                        <Icon name="heroicons:heart-solid" size="10" class="text-pink-400" />
                        {{ playlist.like_count }}
                    </span>
                </span>
                <span class="text-dim">{{ playlist.owner.username }}</span>
            </div>
            <!-- Tags -->
            <div v-if="playlist.tags.length > 0" class="mt-2 flex flex-wrap gap-1">
                <span
                    v-for="tag in playlist.tags.slice(0, 3)"
                    :key="tag.id"
                    class="rounded px-1.5 py-0.5 text-[9px] font-medium"
                    :style="{ backgroundColor: tag.color + '20', color: tag.color }"
                >
                    {{ tag.name }}
                </span>
                <span
                    v-if="playlist.tags.length > 3"
                    class="text-dim rounded bg-white/5 px-1.5 py-0.5 text-[9px]"
                >
                    +{{ playlist.tags.length - 3 }}
                </span>
            </div>
        </div>
    </NuxtLink>
</template>
