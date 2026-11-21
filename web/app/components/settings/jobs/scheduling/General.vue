<script setup lang="ts">
import type { JobScheduleConfig } from '~/types/jobs';

const config = defineModel<JobScheduleConfig>({ required: true });

const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;

const timezones = computed<string[]>(() => {
    const intl = Intl as unknown as { supportedValuesOf?: (key: string) => string[] };
    const list = intl.supportedValuesOf ? intl.supportedValuesOf('timeZone') : [];
    return list.includes('UTC') ? list : ['UTC', ...list];
});

const clamp = (val: number, min: number, max: number) =>
    Math.max(min, Math.min(max, Number.isFinite(val) ? Math.round(val) : min));

const crossesMidnight = computed(
    () => config.value.quiet_hours_end < config.value.quiet_hours_start,
);

const inputClass =
    'border-border bg-surface rounded-lg border px-2 py-1.5 text-xs text-white [color-scheme:dark] focus:border-white/20 focus:outline-none';
</script>

<template>
    <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <!-- Timezone -->
        <div>
            <label class="mb-1 flex items-center gap-1.5 text-xs font-medium text-white">
                <Icon name="heroicons:globe-alt" size="13" class="text-dim" />
                Timezone
            </label>
            <p class="text-dim mb-2 text-[10px]">Time windows and quiet hours use this clock.</p>
            <div class="flex items-center gap-2">
                <input
                    v-model.trim="config.timezone"
                    list="job-schedule-timezones"
                    :class="[inputClass, 'w-56']"
                    placeholder="UTC"
                />
                <datalist id="job-schedule-timezones">
                    <option v-for="tz in timezones" :key="tz" :value="tz" />
                </datalist>
                <button
                    v-if="browserTimezone && config.timezone !== browserTimezone"
                    type="button"
                    class="rounded-lg bg-white/5 px-2.5 py-1.5 text-[11px] text-white
                        transition-colors hover:bg-white/10"
                    @click="config.timezone = browserTimezone"
                >
                    Use {{ browserTimezone }}
                </button>
            </div>
        </div>

        <!-- Quiet hours -->
        <div>
            <label class="mb-1 flex items-center gap-1.5 text-xs font-medium text-white">
                <Icon name="heroicons:moon" size="13" class="text-dim" />
                Quiet Hours
            </label>
            <p class="text-dim mb-2 text-[10px]">
                When the quiet hours worker counts below apply.
                <template v-if="crossesMidnight">Crosses midnight.</template>
            </p>
            <div class="flex items-center gap-2">
                <input v-model="config.quiet_hours_start" type="time" :class="inputClass" />
                <span class="text-dim text-xs">to</span>
                <input v-model="config.quiet_hours_end" type="time" :class="inputClass" />
            </div>
        </div>

        <!-- Global cap -->
        <div>
            <label class="mb-1 flex items-center gap-1.5 text-xs font-medium text-white">
                <Icon name="heroicons:cpu-chip" size="13" class="text-dim" />
                Global Limit
            </label>
            <p class="text-dim mb-2 text-[10px]">
                Maximum jobs running at once across all phases. 0 = no limit.
            </p>
            <input
                v-model.number="config.max_concurrent_jobs"
                type="number"
                min="0"
                max="256"
                :class="[inputClass, 'w-20 text-center']"
                @change="config.max_concurrent_jobs = clamp(config.max_concurrent_jobs, 0, 256)"
            />
        </div>

        <!-- New scenes first -->
        <div>
            <div class="mb-1 flex items-center justify-between gap-3">
                <label class="flex items-center gap-1.5 text-xs font-medium text-white">
                    <Icon name="heroicons:sparkles" size="13" class="text-dim" />
                    New Scenes First
                </label>
                <UiToggle v-model="config.new_scenes_first" />
            </div>
            <p class="text-dim mb-2 text-[10px]">
                Jobs of scenes added recently go before the older jobs of the same phase. Imported
                scenes count from the creation date they were imported with.
            </p>
            <div class="flex items-center gap-2">
                <span class="text-dim text-[11px]">Added in the last</span>
                <input
                    v-model.number="config.new_scenes_window_hours"
                    type="number"
                    min="1"
                    max="720"
                    :disabled="!config.new_scenes_first"
                    :class="[inputClass, 'w-20 text-center disabled:opacity-50']"
                    @change="
                        config.new_scenes_window_hours = clamp(
                            config.new_scenes_window_hours,
                            1,
                            720,
                        )
                    "
                />
                <span class="text-dim text-[11px]">hours</span>
            </div>
        </div>
    </div>
</template>
