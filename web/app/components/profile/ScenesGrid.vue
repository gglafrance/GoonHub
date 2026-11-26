<script setup lang="ts">
import type { ListFigureKind, ProfileScene, ProfileSceneFigureKind } from '~/types/profile';

// Scene cards of a profile block, with the figure of the block under each card (watched or
// liked date, view count, rating, O count), or the figures of a custom list (`listFigures`,
// the kinds sent by the server). `showcase` makes bigger cards (favorite videos).
// The columns follow the width of the block (a narrow layout column gets fewer, wider cards).
const props = defineProps<{
    scenes: ProfileScene[];
    figure?: ProfileSceneFigureKind;
    atLabel?: string;
    listFigures?: ListFigureKind[];
    showcase?: boolean;
}>();
</script>

<template>
    <div
        class="grid gap-3"
        :class="
            props.showcase
                ? 'grid-cols-[repeat(auto-fill,minmax(10rem,1fr))]'
                : 'grid-cols-[repeat(auto-fill,minmax(9rem,1fr))]'
        "
    >
        <div v-for="item in props.scenes" :key="item.scene.id" class="min-w-0">
            <SceneCard :scene="item.scene" fluid />
            <ProfileListFigures
                v-if="props.listFigures"
                :kinds="props.listFigures"
                :figures="item.figures"
            />
            <ProfileSceneFigure
                v-else
                :item="item"
                :figure="props.figure"
                :at-label="props.atLabel"
            />
        </div>
    </div>
</template>
