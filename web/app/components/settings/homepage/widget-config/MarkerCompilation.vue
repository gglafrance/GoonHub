<script setup lang="ts">
// Config form of the marker_compilation widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const clipSeconds = computed({
    get: () => (typeof config.value.clip_seconds === 'number' ? config.value.clip_seconds : 15),
    set: (v: number) =>
        (config.value = {
            ...config.value,
            clip_seconds: Math.min(120, Math.max(3, Math.round(Number(v) || 15))),
        }),
});
</script>

<template>
    <div class="space-y-3">
        <SettingsHomepageWidgetConfigMarkersMarkerFilters v-model="config" with-order />
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Clip length without end
            </label>
            <div class="flex items-center gap-2">
                <input
                    v-model.number="clipSeconds"
                    type="number"
                    min="3"
                    max="120"
                    class="border-border bg-void/80 focus:border-lava/40 w-20 rounded-lg border px-3
                        py-2 text-xs text-white focus:outline-none"
                />
                <span class="text-dim text-[11px]">
                    seconds played for markers without an end time
                </span>
            </div>
        </div>
    </div>
</template>
