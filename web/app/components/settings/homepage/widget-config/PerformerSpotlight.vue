<script setup lang="ts">
// Config form of the performer_spotlight widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const order = computed({
    get: () => String(config.value.order ?? 'views'),
    set: (v: string) => (config.value = { ...config.value, order: v }),
});

const orders = [
    { value: 'views', label: 'Most watched' },
    { value: 'jizz', label: 'Most jizz' },
    { value: 'scenes', label: 'Most scenes' },
];
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            Rank by
        </label>
        <div class="flex gap-1.5">
            <button
                v-for="option in orders"
                :key="option.value"
                class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                :class="
                    order === option.value
                        ? 'border-lava/40 bg-lava/10 text-lava'
                        : 'border-border text-dim hover:text-white'
                "
                @click="order = option.value"
            >
                {{ option.label }}
            </button>
        </div>
    </div>
</template>
