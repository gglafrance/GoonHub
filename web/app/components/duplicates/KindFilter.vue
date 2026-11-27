<script setup lang="ts">
// Filter of the duplicate groups by kind (how their scenes relate to each other).
defineProps<{
    modelValue: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string];
}>();

const kinds = [
    { value: '', label: 'All kinds', title: 'Every group' },
    {
        value: 'identical',
        label: 'Identical',
        title: 'Every scene is the same video (safe for auto-resolve)',
    },
    {
        value: 'containment',
        label: 'Containment',
        title: 'A scene is a clip of another one (e.g. part of a compilation)',
    },
    { value: 'partial', label: 'Partial', title: 'Only a minor part of the scenes match' },
    {
        value: 'unverified',
        label: 'Unverified',
        title: 'No matching segment recorded yet for some scenes',
    },
];
</script>

<template>
    <div class="border-border flex gap-0.5 rounded-md border p-0.5">
        <button
            v-for="k in kinds"
            :key="k.value"
            class="rounded px-2 py-0.5 text-[11px] font-medium transition-all"
            :class="
                modelValue === k.value
                    ? 'bg-white/10 text-white'
                    : 'text-dim hover:bg-white/5 hover:text-white'
            "
            :title="k.title"
            @click="emit('update:modelValue', k.value)"
        >
            {{ k.label }}
        </button>
    </div>
</template>
