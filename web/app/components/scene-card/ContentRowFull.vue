<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

const props = defineProps<{
    field: string;
    mode?: 'short' | 'long';
    scene: SceneListItem;
    rating?: number;
    jizzCount?: number;
}>();

const { formatSize, formatFrameRate } = useFormatter();

const metaValue = computed(() => {
    switch (props.field) {
        case 'file_size':
            return formatSize(props.scene.size);
        case 'added_at':
            return null; // handled in template with NuxtTime
        case 'views':
            return props.scene.view_count != null ? `${props.scene.view_count} views` : '';
        case 'resolution':
            return props.scene.height ? `${props.scene.width}x${props.scene.height}` : '';
        case 'frame_rate':
            return props.scene.frame_rate ? formatFrameRate(props.scene.frame_rate) : '';
        case 'jizz_count':
            return props.jizzCount != null ? `${props.jizzCount}` : '0';
        case 'rating':
            return props.rating ? `${props.rating.toFixed(1)}` : '';
        default:
            return '';
    }
});

const isMetaField = computed(() =>
    ['file_size', 'added_at', 'views', 'resolution', 'frame_rate', 'jizz_count', 'rating'].includes(props.field),
);
</script>

<template>
    <!-- Description -->
    <p
        v-if="field === 'description' && scene.description"
        class="text-dim truncate text-[10px]"
        :title="scene.description"
    >
        {{ scene.description }}
    </p>

    <!-- Studio -->
    <NuxtLink
        v-else-if="field === 'studio' && scene.studio"
        :to="{ path: '/', query: { studio: scene.studio } }"
        target="_blank"
        class="text-dim hover:text-lava inline-block truncate text-[10px] transition-colors"
        @click.stop
    >
        {{ scene.studio }}
    </NuxtLink>

    <!-- Tags short mode -->
    <SceneCardTagsShort
        v-else-if="field === 'tags' && mode === 'short' && scene.tags?.length"
        :tags="scene.tags"
    />

    <!-- Tags long mode -->
    <SceneCardTagsLong
        v-else-if="field === 'tags' && mode !== 'short' && scene.tags?.length"
        :tags="scene.tags"
    />

    <!-- Actors short mode -->
    <SceneCardActorsShort
        v-else-if="field === 'actors' && mode === 'short' && scene.actors?.length"
        :actors="scene.actors"
    />

    <!-- Actors long mode -->
    <SceneCardActorsLong
        v-else-if="field === 'actors' && mode !== 'short' && scene.actors?.length"
        :actors="scene.actors"
    />

    <!-- Simple meta fields -->
    <div v-else-if="isMetaField" class="text-dim flex items-center gap-0.5 font-mono text-[10px]">
        <NuxtTime v-if="field === 'added_at'" :datetime="scene.created_at" format="short" />
        <template v-else-if="field === 'jizz_count'">
            <Icon name="fluent-emoji-high-contrast:sweat-droplets" size="11" />
            <span>{{ metaValue }}</span>
        </template>
        <span v-else>{{ metaValue }}</span>
    </div>
</template>
