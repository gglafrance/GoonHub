<script setup lang="ts">
// Config form of the trends widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const set = (key: string, value: unknown) => (config.value = { ...config.value, [key]: value });

const kind = computed({
    get: () => String(config.value.kind ?? 'both'),
    set: (v: string) => set('kind', v),
});

const numberField = (key: string, def: number, min: number, max: number) =>
    computed({
        get: () => Number(config.value[key] ?? def),
        set: (v: number) => set(key, Math.min(max, Math.max(min, Math.round(Number(v) || def)))),
    });

const recentDays = numberField('recent_days', 30, 1, 365);
const baselineDays = numberField('baseline_days', 180, 1, 1825);
const minViews = numberField('min_views', 2, 1, 1000);

const kinds = [
    { value: 'both', label: 'Tags & actors' },
    { value: 'tags', label: 'Tags' },
    { value: 'actors', label: 'Actors' },
];
</script>

<template>
    <div class="space-y-3">
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Show
            </label>
            <div class="flex gap-1.5">
                <button
                    v-for="option in kinds"
                    :key="option.value"
                    class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                    :class="
                        kind === option.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="kind = option.value"
                >
                    {{ option.label }}
                </button>
            </div>
        </div>
        <div class="grid grid-cols-3 gap-3">
            <div>
                <label class="text-dim mb-1.5 block text-[10px] tracking-wider uppercase">
                    Recent days
                </label>
                <input
                    v-model.number="recentDays"
                    type="number"
                    min="1"
                    max="365"
                    class="border-border bg-void/80 w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1.5 block text-[10px] tracking-wider uppercase">
                    Compared to (days)
                </label>
                <input
                    v-model.number="baselineDays"
                    type="number"
                    min="1"
                    max="1825"
                    class="border-border bg-void/80 w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1.5 block text-[10px] tracking-wider uppercase">
                    Min. recent views
                </label>
                <input
                    v-model.number="minViews"
                    type="number"
                    min="1"
                    max="1000"
                    class="border-border bg-void/80 w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
        </div>
        <p class="text-dim text-[10px]">
            Ranks tags and actors by how much their share of your views grew over the recent days,
            compared to the period just before.
        </p>
    </div>
</template>
