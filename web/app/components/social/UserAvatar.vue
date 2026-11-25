<script setup lang="ts">
const props = withDefaults(defineProps<{ username: string; size?: 'sm' | 'lg' }>(), {
    size: 'sm',
});

const initial = computed(() => props.username.charAt(0).toUpperCase() || '?');
</script>

<template>
    <div
        class="border-border bg-panel flex shrink-0 items-center justify-center rounded-full border"
        :class="size === 'lg' ? 'h-14 w-14' : 'h-8 w-8'"
    >
        <span class="font-semibold text-white" :class="size === 'lg' ? 'text-xl' : 'text-xs'">
            {{ initial }}
        </span>
    </div>
</template>
