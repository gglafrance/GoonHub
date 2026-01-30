<script setup lang="ts">
const model = defineModel<boolean>({ required: true });

defineProps<{
    disabled?: boolean;
}>();
</script>

<template>
    <button
        type="button"
        @click="!disabled && (model = !model)"
        class="relative h-5 w-9 rounded-full transition-colors"
        :class="[
            model ? 'bg-emerald' : 'bg-border',
            disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer',
        ]"
        :disabled="disabled"
    >
        <div
            class="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform"
            :class="model ? 'translate-x-4' : 'translate-x-0.5'"
        ></div>
    </button>
</template>
