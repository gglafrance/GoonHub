<script setup lang="ts">
const props = defineProps<{
    actors: string[];
}>();
</script>

<template>
    <div class="flex flex-wrap gap-1 overflow-hidden" style="max-height: 20px">
        <NuxtLink
            v-for="actor in props.actors"
            :key="actor"
            :to="`/actors/${encodeURIComponent(actor)}`"
            class="text-dim hover:text-lava inline-block truncate text-[10px] transition-colors"
            @click.stop
        >
            {{ actor }}
        </NuxtLink>
    </div>
</template>
