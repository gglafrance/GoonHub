<script setup lang="ts">
// Empty state of the scene widgets.
defineProps<{
    message: string;
    icon?: string;
}>();
</script>

<template>
    <div
        class="border-border flex h-32 flex-col items-center justify-center gap-2 rounded-lg border
            border-dashed text-center"
    >
        <Icon :name="icon ?? 'heroicons:film'" size="20" class="text-dim" />
        <p class="text-dim max-w-sm text-xs">{{ message }}</p>
    </div>
</template>
