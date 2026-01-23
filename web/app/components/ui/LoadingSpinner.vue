<script setup lang="ts">
defineProps<{
    size?: 'sm' | 'md';
    label?: string;
}>();
</script>

<template>
    <div class="flex flex-col items-center gap-3">
        <div
            class="animate-spin rounded-full border-2"
            :class="[size === 'sm' ? 'h-5 w-5' : 'h-6 w-6', 'border-border border-t-lava']"
        ></div>
        <span v-if="label" class="text-dim text-[11px]">{{ label }}</span>
    </div>
</template>
