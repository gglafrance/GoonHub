<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    left: string;
    right: string;
    leftMode?: 'short' | 'long';
    rightMode?: 'short' | 'long';
    scene: SceneListItem;
    rating?: number;
    jizzCount?: number;
}>();
</script>

<template>
    <div class="text-dim flex items-center justify-between font-mono text-[10px]">
        <SceneCardContentRowFull :field="left" :mode="leftMode" :scene="scene" :rating="rating" :jizz-count="jizzCount" />
        <SceneCardContentRowFull :field="right" :mode="rightMode" :scene="scene" :rating="rating" :jizz-count="jizzCount" />
    </div>
</template>
