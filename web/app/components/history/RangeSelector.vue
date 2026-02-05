<script setup lang="ts">
defineProps<{
    modelValue: number;
    mode: 'preset' | 'custom';
    customSince: string;
    customUntil: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [days: number];
    'update:mode': [mode: 'preset' | 'custom'];
    'update:customSince': [date: string];
    'update:customUntil': [date: string];
}>();

const presets = [
    { label: '7 Days', value: 7 },
    { label: '14 Days', value: 14 },
    { label: '30 Days', value: 30 },
    { label: '90 Days', value: 90 },
    { label: '1 Year', value: 365 },
    { label: 'All Time', value: 0 },
];

const selectPreset = (value: number) => {
    emit('update:mode', 'preset');
    emit('update:modelValue', value);
};

const selectCustom = () => {
    emit('update:mode', 'custom');
};
</script>

<template>
    <div class="space-y-2">
        <div class="flex flex-wrap gap-1.5">
            <button
                v-for="opt in presets"
                :key="opt.value"
                class="rounded-full border px-3 py-1 text-xs font-medium transition-all"
                :class="
                    mode === 'preset' && modelValue === opt.value
                        ? 'border-lava/40 bg-lava/10 text-lava'
                        : `border-border bg-surface text-dim hover:border-border-hover
                            hover:text-muted`
                "
                @click="selectPreset(opt.value)"
            >
                {{ opt.label }}
            </button>
            <button
                class="rounded-full border px-3 py-1 text-xs font-medium transition-all"
                :class="
                    mode === 'custom'
                        ? 'border-lava/40 bg-lava/10 text-lava'
                        : `border-border bg-surface text-dim hover:border-border-hover
                            hover:text-muted`
                "
                @click="selectCustom"
            >
                Custom
            </button>
        </div>

        <!-- Custom date inputs -->
        <div v-if="mode === 'custom'" class="flex items-center gap-2">
            <input
                :value="customSince"
                type="date"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
                @input="emit('update:customSince', ($event.target as HTMLInputElement).value)"
            />
            <span class="text-dim text-xs">to</span>
            <input
                :value="customUntil"
                type="date"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
                @input="emit('update:customUntil', ($event.target as HTMLInputElement).value)"
            />
        </div>
    </div>
</template>
