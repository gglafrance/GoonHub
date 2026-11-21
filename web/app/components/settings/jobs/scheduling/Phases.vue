<script setup lang="ts">
import type { PhaseScheduleConfig } from '~/types/jobs';

const phases = defineModel<PhaseScheduleConfig[]>({ required: true });

const props = defineProps<{
    visiblePhases: string[];
    configuredWorkers: Record<string, number>;
}>();

const { phaseLabel, phaseIcon } = useJobFormatting();

const shown = computed(() => phases.value.filter((p) => props.visiblePhases.includes(p.phase)));

const clamp = (val: number, min: number, max: number) =>
    Math.max(min, Math.min(max, Number.isFinite(val) ? Math.round(val) : min));

const inputClass =
    'border-border bg-surface rounded-lg border px-2 py-1.5 text-xs text-white [color-scheme:dark] focus:border-white/20 focus:outline-none disabled:opacity-40';
</script>

<template>
    <div>
        <h4 class="mb-1 text-xs font-semibold text-white">Phases</h4>
        <p class="text-dim mb-3 text-[10px]">
            Priority 0-100 (higher first). A time window crosses midnight when it ends before it
            starts. Quiet hours workers: 0 keeps the Workers tab value.
        </p>

        <div class="space-y-2">
            <div
                v-for="p in shown"
                :key="p.phase"
                class="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-lg border border-white/5
                    bg-white/2 px-4 py-3"
            >
                <div class="flex w-32 items-center gap-2">
                    <Icon :name="phaseIcon(p.phase)" size="14" class="text-dim" />
                    <span class="text-xs font-medium text-white">{{ phaseLabel(p.phase) }}</span>
                </div>

                <!-- Priority -->
                <label class="flex items-center gap-2">
                    <span class="text-dim text-[10px] font-medium">Priority</span>
                    <input
                        v-model.number="p.priority"
                        type="number"
                        min="0"
                        max="100"
                        :class="[inputClass, 'w-16 text-center']"
                        @change="p.priority = clamp(p.priority, 0, 100)"
                    />
                </label>

                <!-- Time window -->
                <div class="flex items-center gap-2">
                    <UiToggle v-model="p.window_enabled" />
                    <span class="text-dim text-[10px] font-medium">Only from</span>
                    <input
                        v-model="p.window_start"
                        type="time"
                        :disabled="!p.window_enabled"
                        :class="inputClass"
                    />
                    <span class="text-dim text-[10px]">to</span>
                    <input
                        v-model="p.window_end"
                        type="time"
                        :disabled="!p.window_enabled"
                        :class="inputClass"
                    />
                </div>

                <!-- Quiet hours workers -->
                <label class="flex items-center gap-2">
                    <span class="text-dim text-[10px] font-medium">Quiet workers</span>
                    <input
                        v-model.number="p.quiet_workers"
                        type="number"
                        min="0"
                        max="32"
                        :class="[inputClass, 'w-16 text-center']"
                        @change="p.quiet_workers = clamp(p.quiet_workers, 0, 32)"
                    />
                    <span class="text-dim text-[10px]">
                        day: {{ props.configuredWorkers[p.phase] ?? '-' }}
                    </span>
                </label>
            </div>
        </div>
    </div>
</template>
