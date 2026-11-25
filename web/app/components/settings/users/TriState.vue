<script setup lang="ts">
import type { PermissionChoice } from '~/types/permissions';

// Segmented Inherit / Allow / Deny control of a permission setting.
const props = defineProps<{
    modelValue: PermissionChoice;
    disabled?: boolean;
    // Label of the neutral choice ("Inherit" for roles, "Role" for user overrides)
    inheritLabel?: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: PermissionChoice];
}>();

const options = computed<{ value: PermissionChoice; label: string; active: string }[]>(() => [
    { value: 'inherit', label: props.inheritLabel ?? 'Inherit', active: 'bg-white/10 text-white' },
    { value: 'allow', label: 'Allow', active: 'bg-emerald/15 text-emerald' },
    { value: 'deny', label: 'Deny', active: 'bg-lava/15 text-lava' },
]);

const select = (value: PermissionChoice) => {
    if (!props.disabled && value !== props.modelValue) emit('update:modelValue', value);
};
</script>

<template>
    <div
        class="border-border bg-void/60 inline-flex shrink-0 rounded-md border p-0.5"
        role="radiogroup"
    >
        <button
            v-for="opt in options"
            :key="opt.value"
            type="button"
            role="radio"
            :aria-checked="modelValue === opt.value"
            :disabled="disabled"
            class="rounded px-2 py-0.5 text-[10px] font-medium transition-colors
                disabled:cursor-not-allowed"
            :class="
                modelValue === opt.value
                    ? opt.active
                    : 'text-dim disabled:hover:text-dim hover:text-white'
            "
            @click="select(opt.value)"
        >
            {{ opt.label }}
        </button>
    </div>
</template>
