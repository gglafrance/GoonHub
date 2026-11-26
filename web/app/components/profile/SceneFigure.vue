<script setup lang="ts">
import type { ProfileScene, ProfileSceneFigureKind } from '~/types/profile';

// The owner's figure under a scene card of a profile block or list: when it was watched or
// liked, the view count, the owner's rating (stars and date) or O count (and last date).
defineProps<{
    item: ProfileScene;
    figure?: ProfileSceneFigureKind;
    atLabel?: string;
}>();
</script>

<template>
    <div v-if="figure === 'rating' && item.rating" class="mt-1 flex min-w-0 items-center gap-1.5">
        <span class="text-lava text-xs leading-none" :title="ratingLabel(item.rating)">{{
            ratingStars(item.rating)
        }}</span>
        <span v-if="item.at" class="text-dim truncate text-[10px]">
            <NuxtTime :datetime="item.at" relative />
        </span>
    </div>
    <div
        v-else-if="figure === 'jizz' && item.jizz_count"
        class="text-dim mt-1 flex min-w-0 items-center gap-1 text-[10px]"
    >
        <Icon name="fluent-emoji-high-contrast:sweat-droplets" size="11" class="text-lava" />
        <span class="font-mono text-white">{{ item.jizz_count }}</span>
        <span v-if="item.at" class="truncate">
            · last <NuxtTime :datetime="item.at" relative />
        </span>
    </div>
    <div v-else-if="figure === 'at' && item.at" class="text-dim mt-1 truncate text-[10px]">
        {{ atLabel }} <NuxtTime :datetime="item.at" relative />
    </div>
    <div v-else-if="figure === 'views' && item.views" class="text-dim mt-1 text-[10px]">
        <span class="font-mono text-white">{{ item.views }}</span>
        {{ item.views === 1 ? 'view' : 'views' }}
    </div>
</template>
