<script setup lang="ts">
defineProps<{
    modelValue: string;
    options: { value: string; label: string }[];
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string];
}>();

const onChange = (event: Event) => {
    emit('update:modelValue', (event.target as HTMLSelectElement).value);
};
</script>

<template>
    <div class="relative">
        <select
            :value="modelValue"
            class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20 h-full
                cursor-pointer appearance-none rounded-lg border py-2 pr-8 pl-3 text-xs text-white
                transition-all focus:ring-2 focus:outline-none"
            @change="onChange"
        >
            <option v-for="opt in options" :key="opt.value" :value="opt.value">
                {{ opt.label }}
            </option>
        </select>
        <Icon
            name="heroicons:chevron-down"
            size="14"
            class="text-dim pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2"
        />
    </div>
</template>
