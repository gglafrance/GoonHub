<script setup lang="ts">
// Discreet button opening the advanced marker filters, with the active filter count.
defineProps<{
    open: boolean;
    activeCount: number;
}>();

const emit = defineEmits<{
    toggle: [];
}>();
</script>

<template>
    <button
        class="border-border bg-panel flex shrink-0 items-center gap-1.5 rounded-lg border px-2.5
            text-[11px] font-medium transition-all"
        :class="open || activeCount > 0 ? 'text-lava border-lava/30' : 'text-dim hover:text-white'"
        title="Advanced marker filters"
        @click="emit('toggle')"
    >
        <Icon name="heroicons:funnel" size="14" />
        <span
            v-if="activeCount > 0"
            class="bg-lava/20 text-lava rounded-full px-1.5 font-mono text-[10px]"
        >
            {{ activeCount }}
        </span>
    </button>
</template>
