<script setup lang="ts">
import type { ListFigureKind, ProfileScene } from '~/types/profile';

// Compact card of a custom list (block and Browse page): the thumbnail with the duration, the
// title, and the owner's figures the list shows (O count, rating, like date, views) as badges on
// the thumbnail. The icon tells what a figure is, the tooltip gives the details.
const props = defineProps<{ item: ProfileScene; figures?: ListFigureKind[] }>();

const { formatDuration } = useFormatter();
const thumbFailed = ref(false);

const scene = computed(() => props.item.scene);
const thumbnailUrl = computed(() => {
    if (!scene.value.thumbnail_path) return null;
    const base = `/thumbnails/${scene.value.id}`;
    const v = scene.value.updated_at ? new Date(scene.value.updated_at).getTime() : '';
    return v ? `${base}?v=${v}` : base;
});
const badges = computed(() => listFigureBadges(props.figures, props.item.figures));
</script>

<template>
    <NuxtLink
        :to="`/watch/${scene.id}`"
        class="group border-border bg-surface hover:border-lava/30 block min-w-0 overflow-hidden
            rounded-lg border transition-colors"
        :title="scene.title"
    >
        <div class="relative aspect-video overflow-hidden bg-black/50">
            <img
                v-if="thumbnailUrl && !thumbFailed"
                :src="thumbnailUrl"
                alt=""
                loading="lazy"
                class="h-full w-full object-cover transition-transform duration-300
                    group-hover:scale-[1.03]"
                @error="thumbFailed = true"
            />
            <div v-else class="text-dim flex h-full w-full items-center justify-center">
                <Icon name="heroicons:play" size="24" />
            </div>
            <div v-if="badges.length" class="absolute top-1.5 left-1.5 flex flex-wrap gap-1">
                <span
                    v-for="badge in badges"
                    :key="badge.kind"
                    class="border-lava/30 flex items-center gap-1 rounded-md border bg-black/75
                        px-1.5 py-0.5 font-mono text-[10px] font-semibold text-white
                        backdrop-blur-sm"
                    :title="badge.title"
                >
                    <Icon :name="badge.icon" size="11" class="text-lava shrink-0" />
                    {{ badge.text }}
                </span>
            </div>
            <span
                v-if="scene.duration"
                class="absolute right-1.5 bottom-1.5 rounded bg-black/75 px-1.5 py-0.5 font-mono
                    text-[10px] text-white"
            >
                {{ formatDuration(scene.duration) }}
            </span>
        </div>
        <div
            class="group-hover:text-lava truncate px-2.5 py-2 text-xs font-medium text-white
                transition-colors"
        >
            {{ scene.title }}
        </div>
    </NuxtLink>
</template>
