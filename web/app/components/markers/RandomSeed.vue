<script setup lang="ts">
// Seed of the random marker sort (kept in the URL), with a button drawing a new order.
defineProps<{
    seed: number;
}>();

const emit = defineEmits<{
    reshuffle: [];
}>();
</script>

<template>
    <button
        class="border-border bg-panel hover:border-lava/40 hover:bg-lava/10 group flex shrink-0
            items-center gap-1.5 rounded-lg border px-2.5 transition-all"
        :title="`Random seed ${seed} - click to reshuffle`"
        @click="emit('reshuffle')"
    >
        <Icon
            name="heroicons:arrow-path"
            size="14"
            class="text-dim transition-colors group-hover:text-white"
        />
        <span class="text-dim hidden font-mono text-[10px] sm:inline">{{ seed }}</span>
    </button>
</template>
