<script setup lang="ts">
const props = defineProps<{
    title: string;
    icon: string;
    modelValue: string;
    options: { value: string; label: string }[];
    placeholder?: string;
    defaultCollapsed?: boolean;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string];
}>();

const collapsed = ref(props.defaultCollapsed ?? true);

const badge = computed(() => (props.modelValue ? props.modelValue : undefined));
</script>

<template>
    <SearchFiltersFilterSection
        :title="title"
        :icon="icon"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <select
            :value="modelValue"
            class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5 text-xs
                focus:border-white/20 focus:outline-none"
            @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
        >
            <option v-if="placeholder" value="">{{ placeholder }}</option>
            <option v-for="opt in options" :key="opt.value" :value="opt.value">
                {{ opt.label }}
            </option>
        </select>
    </SearchFiltersFilterSection>
</template>
