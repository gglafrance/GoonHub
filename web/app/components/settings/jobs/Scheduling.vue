<script setup lang="ts">
import type { JobScheduleConfig, JobScheduleStatus } from '~/types/jobs';

const { fetchJobSchedule, updateJobSchedule } = useApiJobs();

const loading = ref(true);
const saving = ref(false);
const error = ref('');
const message = ref('');

const config = ref<JobScheduleConfig | null>(null);
const status = ref<JobScheduleStatus | null>(null);

// Phases with a worker pool (fingerprint only with duplicate detection)
const visiblePhases = computed(() => status.value?.phases.map((p) => p.phase) ?? []);
const configuredWorkers = computed(() => {
    const map: Record<string, number> = {};
    for (const p of status.value?.phases ?? []) map[p.phase] = p.configured_workers;
    return map;
});

const load = async () => {
    loading.value = true;
    error.value = '';
    try {
        const res = await fetchJobSchedule();
        config.value = res.config;
        status.value = res.status;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load the job schedule';
    } finally {
        loading.value = false;
    }
};

// Only the effective state is refreshed: the form keeps unsaved edits.
const refreshStatus = async () => {
    try {
        const res = await fetchJobSchedule();
        status.value = res.status;
    } catch {
        // keep the last known state
    }
};

const save = async () => {
    if (!config.value) return;
    saving.value = true;
    error.value = '';
    message.value = '';
    try {
        const res = await updateJobSchedule(config.value);
        config.value = res.config;
        status.value = res.status;
        message.value = 'Job schedule saved. It applies within a few seconds.';
        setTimeout(() => {
            message.value = '';
        }, 4000);
        // The feeder applies pool sizes on its next tick
        setTimeout(refreshStatus, 3000);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the job schedule';
    } finally {
        saving.value = false;
    }
};

let timer: ReturnType<typeof setInterval> | undefined;

onMounted(() => {
    load();
    timer = setInterval(refreshStatus, 15000);
});

onUnmounted(() => {
    if (timer) clearInterval(timer);
});
</script>

<template>
    <div class="space-y-5">
        <SettingsJobsSchedulingStatus v-if="status" :status="status" />

        <div class="glass-panel p-5">
            <div class="mb-4">
                <h3 class="text-sm font-semibold text-white">Scheduling</h3>
                <p class="text-dim mt-1 text-[11px]">
                    Decide when and in which order queued jobs run. Jobs are still created as usual
                    (imports, triggers, bulk actions); scheduling only delays when they start.
                </p>
            </div>

            <div
                v-if="error"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ error }}
            </div>

            <div
                v-if="message"
                class="mb-4 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2
                    text-xs text-emerald-400"
            >
                {{ message }}
            </div>

            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

            <div v-else-if="config" class="space-y-6">
                <SettingsJobsSchedulingGeneral v-model="config" />

                <SettingsJobsSchedulingPhases
                    v-model="config.phases"
                    :visible-phases="visiblePhases"
                    :configured-workers="configuredWorkers"
                />

                <div class="border-border flex items-center justify-end border-t pt-4">
                    <button
                        :disabled="saving"
                        class="bg-lava hover:bg-lava/90 rounded-lg px-4 py-1.5 text-xs font-medium
                            text-white transition-colors disabled:opacity-50"
                        @click="save"
                    >
                        {{ saving ? 'Saving...' : 'Save' }}
                    </button>
                </div>
            </div>
        </div>

        <div class="glass-panel p-4">
            <div class="flex items-start gap-3">
                <Icon name="heroicons:information-circle" size="16" class="text-dim mt-0.5" />
                <div class="text-dim space-y-1 text-[11px]">
                    <p>
                        <strong class="text-white">Time window:</strong> outside its window a phase
                        starts no new job. Its pending jobs wait (they do not fail) and running jobs
                        finish.
                    </p>
                    <p>
                        <strong class="text-white">Priority:</strong> when the global limit is
                        reached, the waiting phase with the highest priority gets the next free
                        slot. Inside a phase, retries go first, then new scenes (if enabled), then
                        the oldest jobs.
                    </p>
                    <p>
                        <strong class="text-white">Quiet hours:</strong> phases with a quiet hours
                        worker count use it during the quiet hours, and their Workers tab value the
                        rest of the day. Changes never interrupt running jobs.
                    </p>
                </div>
            </div>
        </div>
    </div>
</template>
