<script setup lang="ts">
import type { ShortItem } from '~/types/shorts';
import { formatShortSpan } from '~/composables/useShortEditor';

const props = defineProps<{
    item: ShortItem;
    // The short last shown in the player
    focused: boolean;
}>();

const hovering = ref(false);

const version = computed(() =>
    props.item.updated_at ? new Date(props.item.updated_at).getTime() : 0,
);

const thumbnailUrl = computed(() => {
    if (!props.item.thumbnail_path) return null;
    const base = `/thumbnails/${props.item.id}?size=lg`;
    return version.value ? `${base}&v=${version.value}` : base;
});

const previewUrl = computed(() => {
    if (!props.item.preview_video_path) return null;
    const base = `/scene-previews/${props.item.id}`;
    return version.value ? `${base}?v=${version.value}` : base;
});

const isClip = computed(() => props.item.source_start !== null && props.item.source_end !== null);
</script>

<template>
    <button
        type="button"
        class="group relative block aspect-9/16 w-full overflow-hidden rounded-lg border bg-black
            text-left transition-colors"
        :class="focused ? 'border-lava/60' : 'border-border hover:border-white/25'"
        :title="item.title"
        @mouseenter="hovering = true"
        @mouseleave="hovering = false"
        @focus="hovering = true"
        @blur="hovering = false"
    >
        <img
            v-if="thumbnailUrl"
            :src="thumbnailUrl"
            :alt="item.title"
            loading="lazy"
            class="absolute inset-0 h-full w-full object-cover transition-transform duration-300
                group-hover:scale-[1.03]"
        />
        <div v-else class="absolute inset-0 flex items-center justify-center">
            <Icon name="heroicons:film" size="28" class="text-dim" />
        </div>

        <video
            v-if="hovering && previewUrl"
            :src="previewUrl"
            autoplay
            muted
            loop
            playsinline
            class="absolute inset-0 h-full w-full object-cover"
        />

        <span
            class="absolute top-1.5 right-1.5 rounded bg-black/70 px-1.5 py-0.5 font-mono
                text-[10px] text-white"
        >
            {{ formatShortSpan(item.duration) }}
        </span>
        <span
            v-if="isClip"
            class="absolute top-1.5 left-1.5 flex items-center rounded bg-black/70 p-1 text-white"
            title="Created short"
        >
            <Icon name="heroicons:scissors" size="11" />
        </span>

        <div
            class="absolute inset-x-0 bottom-0 bg-linear-to-t from-black/85 via-black/50
                to-transparent px-2 pt-8 pb-2"
        >
            <p class="line-clamp-2 text-xs leading-snug font-semibold text-white drop-shadow">
                {{ item.title }}
            </p>
            <p class="mt-0.5 font-mono text-[10px] text-white/60">{{ item.view_count }} views</p>
        </div>
    </button>
</template>
