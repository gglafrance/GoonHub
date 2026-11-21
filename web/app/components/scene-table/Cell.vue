<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router';
import type { SceneListItem } from '~/types/scene';
import type { SceneTableColumn } from '~/types/settings';

// One cell of a scene table row
const props = defineProps<{
    column: SceneTableColumn;
    scene: SceneListItem;
    rating?: number;
    to: RouteLocationRaw;
    /** Select mode: links do not navigate (the row toggles the selection) */
    selectable?: boolean;
}>();

const { formatDuration, formatSize } = useFormatter();

const thumbnailUrl = computed(() => {
    if (!props.scene.thumbnail_path) return null;
    const v = props.scene.updated_at ? new Date(props.scene.updated_at).getTime() : '';
    return v ? `/thumbnails/${props.scene.id}?v=${v}` : `/thumbnails/${props.scene.id}`;
});

const actorsText = computed(() => props.scene.actors?.join(', ') ?? '');
</script>

<template>
    <NuxtLink
        v-if="column === 'thumbnail'"
        :to="to"
        class="bg-void relative block aspect-video w-16 overflow-hidden rounded"
        :class="selectable ? 'pointer-events-none' : ''"
        :tabindex="-1"
    >
        <img
            v-if="thumbnailUrl"
            :src="thumbnailUrl"
            class="h-full w-full object-cover"
            :alt="scene.title"
            loading="lazy"
        />
        <div v-else class="flex h-full w-full items-center justify-center">
            <Icon name="heroicons:play" size="14" class="text-dim" />
        </div>
    </NuxtLink>

    <div v-else-if="column === 'title'" class="flex min-w-0 items-center gap-1.5">
        <NuxtLink
            :to="to"
            class="truncate font-medium text-white/90 transition-colors hover:text-white"
            :class="selectable ? 'pointer-events-none' : ''"
            :title="scene.title"
        >
            {{ scene.title }}
        </NuxtLink>
        <LoadingSpinner v-if="isSceneProcessing(scene)" size="sm" />
        <Icon
            v-else-if="isSceneCorrupted(scene)"
            name="heroicons:shield-exclamation"
            size="13"
            class="shrink-0 text-amber-400"
            title="Corrupted"
        />
    </div>

    <span v-else-if="column === 'duration'" class="font-mono">
        {{ scene.duration > 0 ? formatDuration(scene.duration) : '-' }}
    </span>

    <span v-else-if="column === 'resolution'" class="font-mono">
        {{ resolutionLabel(scene.height) || '-' }}
    </span>

    <span v-else-if="column === 'file_size'" class="font-mono">
        {{ scene.size > 0 ? formatSize(scene.size) : '-' }}
    </span>

    <span v-else-if="column === 'rating'" class="flex items-center gap-0.5 font-mono">
        <template v-if="rating && rating > 0">
            <Icon name="heroicons:star-solid" size="12" class="text-amber-400" />
            {{ rating.toFixed(1) }}
        </template>
        <template v-else>-</template>
    </span>

    <span v-else-if="column === 'views'" class="font-mono">
        {{ scene.view_count ?? '-' }}
    </span>

    <span v-else-if="column === 'studio'" class="block truncate" :title="scene.studio || undefined">
        {{ scene.studio || '-' }}
    </span>

    <span v-else-if="column === 'actors'" class="block truncate" :title="actorsText || undefined">
        {{ actorsText || '-' }}
    </span>

    <span v-else-if="column === 'release_date'" class="font-mono">
        <UiFormattedDate v-if="scene.release_date" :datetime="scene.release_date" utc />
        <template v-else>-</template>
    </span>

    <span v-else-if="column === 'added_at'" class="font-mono">
        <UiFormattedDate :datetime="scene.created_at" />
    </span>
</template>
