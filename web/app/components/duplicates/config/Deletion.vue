<script setup lang="ts">
// Permanent deletion of the scenes not kept when a duplicate group is resolved.
const enabled = defineModel<boolean>('enabled', { default: false });
const delayHours = defineModel<number>('delayHours', { default: 72 });

const presets = [
    { hours: 0, label: 'Immediately' },
    { hours: 24, label: '1 day' },
    { hours: 72, label: '3 days' },
    { hours: 168, label: '1 week' },
    { hours: 720, label: '30 days' },
];

const delayLabel = computed(() => {
    const h = delayHours.value ?? 0;
    if (h === 0) return 'at the next cleanup (every 10 minutes)';
    if (h % 24 === 0) return `${h / 24} day${h / 24 > 1 ? 's' : ''} after resolution`;
    return `${h} hour${h > 1 ? 's' : ''} after resolution`;
});
</script>

<template>
    <div class="border-border mt-4 border-t pt-4">
        <div class="flex items-center justify-between">
            <div>
                <h3 class="text-sm font-medium text-white">Delete Files Not Kept</h3>
                <p class="text-dim mt-0.5 text-[10px]">
                    When a group is resolved, the scenes not kept go to the trash. With this on,
                    they are then permanently deleted (scene and video file).
                </p>
            </div>
            <UiToggle v-model="enabled" />
        </div>

        <div v-if="enabled" class="mt-3">
            <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase">
                Delay before deletion
            </label>
            <div class="flex flex-wrap items-center gap-1.5">
                <button
                    v-for="preset in presets"
                    :key="preset.hours"
                    class="rounded-md border px-2 py-1 text-[10px] font-medium transition-all"
                    :class="
                        delayHours === preset.hours
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="delayHours = preset.hours"
                >
                    {{ preset.label }}
                </button>
                <div class="flex items-center gap-1">
                    <input
                        v-model.number="delayHours"
                        type="number"
                        min="0"
                        max="8760"
                        class="border-border bg-panel w-20 rounded-md border px-2 py-1 text-xs
                            text-white"
                    />
                    <span class="text-dim text-[10px]">hours</span>
                </div>
            </div>
            <p class="mt-1.5 text-[10px] text-white/40">
                Files are deleted {{ delayLabel }}. Until then they stay in the trash: restoring a
                scene or clicking "Cancel deletion" on the duplicates page keeps it.
            </p>
        </div>
    </div>
</template>
