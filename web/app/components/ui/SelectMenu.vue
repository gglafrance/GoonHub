<script setup lang="ts">
defineProps<{
    modelValue: string | number;
    options: { value: string | number; label: string }[];
}>();

defineEmits<{
    'update:modelValue': [value: string | number];
}>();
</script>

<template>
    <div class="relative">
        <select
            :value="modelValue"
            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 w-full
                appearance-none rounded-lg border py-2.5 pr-9 pl-3.5 text-sm text-white
                transition-all focus:ring-1 focus:outline-none"
            @change="$emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
        >
            <option v-for="opt in options" :key="opt.value" :value="opt.value" class="bg-panel">
                {{ opt.label }}
            </option>
        </select>
        <Icon
            name="heroicons:chevron-down"
            size="14"
            class="text-dim pointer-events-none absolute top-1/2 right-3 -translate-y-1/2"
        />
    </div>
</template>
