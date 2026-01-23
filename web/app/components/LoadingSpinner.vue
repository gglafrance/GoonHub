<script setup lang="ts">
defineProps<{
    label?: string;
    size?: 'sm' | 'md';
}>();
</script>

<template>
    <div class="flex flex-col items-center gap-2">
        <div
            class="animate-spin rounded-full border-2 border-white/10"
            :class="size === 'sm' ? 'h-5 w-5 border-t-lava/70' : 'h-7 w-7 border-t-lava'"
        ></div>
        <span v-if="label" class="text-dim text-xs">{{ label }}</span>
    </div>
</template>
