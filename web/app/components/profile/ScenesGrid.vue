<script setup lang="ts">
import type { ProfileScene, ProfileSceneFigureKind } from '~/types/profile';

// Scene cards of a profile block, with the figure of the block under each card (watched or
// liked date, view count, rating, O count). `showcase` makes bigger cards (favorite videos).
const props = defineProps<{
    scenes: ProfileScene[];
    figure?: ProfileSceneFigureKind;
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
            <ProfileSceneFigure :item="item" :figure="props.figure" :at-label="props.atLabel" />
        </div>
    </div>
</template>
