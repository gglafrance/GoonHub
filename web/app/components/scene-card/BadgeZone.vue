<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type { BadgeZone } from '~/types/settings';
import type { BadgeCorner } from '~/utils/badgeZoneLayout';

const props = defineProps<{
    zone: BadgeZone;
    corner: BadgeCorner;
    scene: SceneListItem;
    liked?: boolean;
    rating?: number;
    jizzCount?: number;
    completed?: boolean;
}>();

// Original flex layout without columns, grid with columns (see utils/badgeZoneLayout.ts)
const layout = computed(() => badgeZoneLayout(props.zone, props.corner));
</script>

<template>
    <div v-if="zone.items.length > 0" :class="layout.class" :style="layout.style">
        <SceneCardBadgeItem
            v-for="(field, idx) in zone.items"
            :key="idx"
            :field="field"
            :scene="scene"
            :liked="liked"
            :rating="rating"
            :jizz-count="jizzCount"
            :completed="completed"
            :cell="layout.grid"
        />
    </div>
</template>
