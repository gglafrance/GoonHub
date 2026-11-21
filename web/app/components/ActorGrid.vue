<script setup lang="ts">
import type { ActorListItem } from '~/types/actor';

defineProps<{
    actors: ActorListItem[];
}>();

const { entityGridClass } = useGridDensity();
</script>

<template>
    <div class="grid" :class="entityGridClass">
        <ActorCard v-for="actor in actors" :key="actor.id" :actor="actor" />
    </div>
</template>
