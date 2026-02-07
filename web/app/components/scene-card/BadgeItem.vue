<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

const props = defineProps<{
    field: string;
    scene: SceneListItem;
    liked?: boolean;
    rating?: number;
    jizzCount?: number;
    completed?: boolean;
}>();

const { formatDuration, formatSize, formatFrameRate } = useFormatter();

const badge = computed(() => {
    switch (props.field) {
        case 'duration':
            return props.scene.duration > 0
                ? { icon: null, text: formatDuration(props.scene.duration), class: 'bg-void/90' }
                : null;
        case 'file_size':
            return props.scene.size > 0
                ? { icon: null, text: formatSize(props.scene.size), class: 'bg-void/90' }
                : null;
        case 'added_at':
            return { icon: null, text: null, class: 'bg-void/90', isDate: true };
        case 'liked':
            return props.liked
                ? { icon: 'heroicons:heart-solid', text: null, class: 'bg-lava/90 text-white' }
                : null;
        case 'rating':
            return props.rating && props.rating >= 5
                ? {
                      icon: 'heroicons:star-solid',
                      text: null,
                      class: 'bg-amber-400/90 text-amber-900',
                      round: true,
                  }
                : props.rating && props.rating > 0
                  ? {
                        icon: 'heroicons:star-solid',
                        text: props.rating.toFixed(1),
                        class: 'bg-amber-400/90 text-amber-900',
                    }
                  : null;
        case 'resolution':
            if (props.scene.height) {
                const h = props.scene.height;
                const label = h >= 2160 ? '4K' : h >= 1440 ? '1440p' : h >= 1080 ? '1080p' : h >= 720 ? '720p' : h >= 480 ? '480p' : `${h}p`;
                return { icon: null, text: label, class: 'bg-void/90' };
            }
            return null;
        case 'frame_rate':
            return props.scene.frame_rate
                ? { icon: null, text: formatFrameRate(props.scene.frame_rate), class: 'bg-void/90' }
                : null;
        case 'views':
            return props.scene.view_count != null
                ? { icon: 'heroicons:eye', text: String(props.scene.view_count), class: 'bg-void/90' }
                : null;
        case 'jizz_count':
            return props.jizzCount && props.jizzCount > 0
                ? { icon: 'fluent-emoji-high-contrast:sweat-droplets', text: String(props.jizzCount), class: 'bg-void/90' }
                : null;
        case 'watched':
            return props.completed
                ? { icon: null, text: 'Watched', class: 'bg-emerald-500/90 text-white !text-[9px] font-semibold' }
                : null;
        default:
            return null;
    }
});
</script>

<template>
    <!-- Tags badge with popover -->
    <SceneCardTagsShort
        v-if="field === 'tags' && scene.tags?.length"
        :tags="scene.tags"
        badge
    />

    <!-- Actors badge with popover -->
    <SceneCardActorsShort
        v-else-if="field === 'actors' && scene.actors?.length"
        :actors="scene.actors"
        badge
    />

    <!-- Standard badges -->
    <div
        v-else-if="badge"
        class="z-20 flex items-center gap-0.5 rounded px-1.5 py-0.5 font-mono text-[10px] font-medium text-white backdrop-blur-sm"
        :class="[badge.class, badge.round ? 'h-6 w-6 justify-center rounded-full !p-0' : '']"
    >
        <Icon v-if="badge.icon" :name="badge.icon" size="14" />
        <NuxtTime v-if="badge.isDate" :datetime="scene.created_at" format="short" />
        <span v-else-if="badge.text">{{ badge.text }}</span>
    </div>
</template>
