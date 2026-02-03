<script setup lang="ts">
const props = defineProps<{
    modelValue: number;
}>();

const emit = defineEmits<{
    'update:modelValue': [days: number];
}>();

const options = [
    { label: '30 Days', value: 30 },
    { label: '90 Days', value: 90 },
    { label: '1 Year', value: 365 },
    { label: 'All Time', value: 0 },
];
</script>

<template>
    <div class="flex gap-1.5">
        <button
            v-for="opt in options"
            :key="opt.value"
            class="rounded-full border px-3 py-1 text-xs font-medium transition-all"
            :class="
                modelValue === opt.value
                    ? 'border-lava/40 bg-lava/10 text-lava'
                    : 'border-border bg-surface text-dim hover:border-border-hover hover:text-muted'
            "
            @click="emit('update:modelValue', opt.value)"
        >
            {{ opt.label }}
        </button>
    </div>
</template>
