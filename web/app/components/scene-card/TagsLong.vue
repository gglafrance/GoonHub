<script setup lang="ts">
const props = defineProps<{
    tags: string[];
}>();
</script>

<template>
    <div class="flex flex-wrap gap-1 overflow-hidden" style="max-height: 20px">
        <NuxtLink
            v-for="tag in props.tags"
            :key="tag"
            :to="{ path: '/', query: { tags: tag } }"
            target="_blank"
            class="border-border text-dim hover:border-lava/40 hover:text-lava inline-block truncate rounded border px-1 py-px text-[9px] transition-colors"
            @click.stop
        >
            {{ tag }}
        </NuxtLink>
    </div>
</template>
