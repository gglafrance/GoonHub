<script setup lang="ts">
import type { ProfileScene } from '~/types/profile';

// Scene cards of a profile block, with the figure of the block under each card (watched or
// liked date, view count). `showcase` makes bigger cards (favorite videos).
const props = defineProps<{
    scenes: ProfileScene[];
    figure?: 'at' | 'views' | 'none';
    atLabel?: string;
    showcase?: boolean;
}>();
</script>

<template>
    <div
        class="grid gap-3"
        :class="
            props.showcase
                ? 'grid-cols-2 md:grid-cols-4'
                : 'grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6'
        "
    >
        <div v-for="item in props.scenes" :key="item.scene.id" class="min-w-0">
            <SceneCard :scene="item.scene" fluid />
            <div v-if="props.figure === 'at' && item.at" class="text-dim mt-1 truncate text-[10px]">
                {{ props.atLabel }} <NuxtTime :datetime="item.at" relative />
            </div>
            <div
                v-else-if="props.figure === 'views' && item.views"
                class="text-dim mt-1 text-[10px]"
            >
                <span class="font-mono text-white">{{ item.views }}</span>
                {{ item.views === 1 ? 'view' : 'views' }}
            </div>
        </div>
    </div>
</template>
