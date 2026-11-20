<script setup lang="ts">
// Toggle row of the scene widget config forms.
defineProps<{
    label: string;
    description?: string;
}>();

const value = defineModel<boolean>({ required: true });
</script>

<template>
    <div class="flex items-center justify-between gap-3">
        <div>
            <span class="block text-xs font-medium text-white">{{ label }}</span>
            <span v-if="description" class="text-dim text-[11px]">{{ description }}</span>
        </div>
        <UiToggle v-model="value" />
    </div>
</template>
