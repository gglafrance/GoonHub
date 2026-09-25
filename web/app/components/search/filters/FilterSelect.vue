<script setup lang="ts">
const props = defineProps<{
    title: string;
    icon: string;
    modelValue: string;
    options: { value: string; label: string }[];
    placeholder?: string;
    defaultCollapsed?: boolean;
    // Badge shown instead of the raw value while one is selected; '' hides it
    badgeText?: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string];
}>();

const collapsed = ref(props.defaultCollapsed ?? true);

const badge = computed(() => {
    if (!props.modelValue) return undefined;
    if (props.badgeText !== undefined) return props.badgeText || undefined;
    return props.modelValue;
});
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
