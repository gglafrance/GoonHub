<script setup lang="ts">
import type { PlaylistDetail } from '~/types/playlist';

defineProps<{
    playlist: PlaylistDetail;
    isOwner: boolean;
}>();

const emit = defineEmits<{
    edit: [];
    delete: [];
    toggleLike: [];
}>();

const { formatDuration } = useFormatter();
</script>

<template>
    <div class="mb-6 space-y-4">
        <!-- Title & actions -->
        <div class="flex items-start justify-between gap-4">
            <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                    <h1 class="truncate text-lg font-semibold text-white sm:text-xl">
                        {{ playlist.name }}
                    </h1>
                    <span
                        v-if="playlist.visibility !== 'public'"
                        class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold"
                        :class="
                            playlist.visibility === 'private'
                                ? 'bg-amber-500/20 text-amber-400'
                                : 'bg-blue-500/20 text-blue-400'
                        "
                    >
                        {{ playlist.visibility === 'private' ? 'Private' : 'Unlisted' }}
                    </span>
                </div>
                <p v-if="playlist.description" class="text-dim mt-1 text-sm">
                    {{ playlist.description }}
                </p>
            </div>

            <div class="flex shrink-0 items-center gap-2">
                <button
                    class="border-border bg-surface hover:bg-elevated flex items-center gap-1.5
                        rounded-lg border px-3 py-1.5 text-xs transition-all"
                    :class="playlist.is_liked ? 'text-pink-400' : 'text-dim hover:text-pink-400'"
                    @click="emit('toggleLike')"
                >
                    <Icon
                        :name="playlist.is_liked ? 'heroicons:heart-solid' : 'heroicons:heart'"
                        size="14"
                    />
                    {{ playlist.like_count }}
                </button>

                <template v-if="isOwner">
                    <button
                        class="border-border bg-surface text-dim hover:border-border-hover
                            hover:bg-elevated flex items-center gap-1.5 rounded-lg border px-3
                            py-1.5 text-xs transition-all hover:text-white"
                        @click="emit('edit')"
                    >
                        <Icon name="heroicons:pencil-square" size="14" />
                        Edit
                    </button>
                    <button
                        class="border-border bg-surface text-dim hover:border-lava/30
                            hover:text-lava flex items-center gap-1.5 rounded-lg border px-3 py-1.5
                            text-xs transition-all"
                        @click="emit('delete')"
                    >
                        <Icon name="heroicons:trash" size="14" />
                    </button>
                </template>
            </div>
        </div>

        <!-- Stats bar -->
        <div class="text-dim flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
            <span class="flex items-center gap-1">
                <Icon name="heroicons:film" size="14" />
                {{ playlist.scene_count }} {{ playlist.scene_count === 1 ? 'scene' : 'scenes' }}
            </span>
            <span class="flex items-center gap-1">
                <Icon name="heroicons:clock" size="14" />
                {{ formatDuration(Number(playlist.total_duration)) }}
            </span>
            <span class="flex items-center gap-1">
                <Icon name="heroicons:user" size="14" />
                {{ playlist.owner.username }}
            </span>
            <NuxtTime :datetime="playlist.created_at" format="short" class="text-dim" />
        </div>

        <!-- Tags -->
        <div v-if="playlist.tags.length > 0" class="flex flex-wrap gap-1.5">
            <span
                v-for="tag in playlist.tags"
                :key="tag.id"
                class="rounded-md px-2 py-0.5 text-[11px] font-medium"
                :style="{ backgroundColor: tag.color + '20', color: tag.color }"
            >
                {{ tag.name }}
            </span>
        </div>
    </div>
</template>
