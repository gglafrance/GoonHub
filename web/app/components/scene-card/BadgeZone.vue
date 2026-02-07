<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    items: string[];
    direction: 'vertical' | 'horizontal';
    scene: SceneListItem;
    liked?: boolean;
    rating?: number;
    jizzCount?: number;
    completed?: boolean;
}>();
</script>

<template>
    <div
        v-if="items.length > 0"
        class="flex gap-1"
        :class="direction === 'vertical' ? 'flex-col items-start' : 'flex-row items-end'"
    >
        <SceneCardBadgeItem
            v-for="(field, idx) in items"
            :key="idx"
            :field="field"
            :scene="scene"
            :liked="liked"
            :rating="rating"
            :jizz-count="jizzCount"
            :completed="completed"
        />
    </div>
</template>
