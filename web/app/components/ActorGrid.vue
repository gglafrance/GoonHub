<script setup lang="ts">
import type { ActorListItem } from '~/types/actor';

defineProps<{
    actors: ActorListItem[];
}>();
</script>

<template>
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        <ActorCard v-for="actor in actors" :key="actor.id" :actor="actor" />
    </div>
</template>
