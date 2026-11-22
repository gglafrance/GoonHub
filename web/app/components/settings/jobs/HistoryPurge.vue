<script setup lang="ts">
import type { HistoryPurgeInfo, HistoryPurgeResult } from '~/types/jobs';

const emit = defineEmits<{
    purged: [result: HistoryPurgeResult];
}>();

const { fetchHistoryPurge } = useApiJobs();

const info = ref<HistoryPurgeInfo | null>(null);
const error = ref('');
const showModal = ref(false);

const settings = computed(() => info.value?.settings ?? null);
const lastRun = computed(() => info.value?.last_run ?? null);

const load = async () => {
    error.value = '';
    try {
        info.value = await fetchHistoryPurge();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load the purge settings';
    }
};

const handlePurged = (result: HistoryPurgeResult) => {
    showModal.value = false;
    emit('purged', result);
    load();
};

onMounted(load);
</script>

<template>
    <div class="glass-panel p-4">
        <div class="mb-2 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-white">History Retention</h3>
            <button
                :disabled="!settings"
                class="text-dim hover:border-lava/30 hover:text-lava flex items-center gap-1 rounded
                    border border-white/10 px-2 py-0.5 text-[10px] font-medium transition-colors
                    disabled:opacity-40"
                @click="showModal = true"
            >
                <Icon name="heroicons:trash" size="11" />
                Purge now
            </button>
        </div>

        <div v-if="error" class="text-lava text-[11px]">{{ error }}</div>
        <template v-else-if="settings">
            <p class="text-dim text-[11px] leading-relaxed">
                Completed and cancelled jobs are purged after
                <span class="text-white">{{
                    settings.completed_enabled ? settings.completed_retention : 'never'
                }}</span
                >, failed and timed out jobs after
                <span class="text-white">{{
                    settings.failed_enabled ? settings.failed_retention : 'never'
                }}</span
                >. The purge runs every
                <span class="text-white">{{ settings.interval }}</span>
                ({{ settings.batch_size.toLocaleString() }} rows per batch). Pending and running
                jobs and the dead letter queue are never purged.
            </p>
            <p class="mt-1 text-[10px] text-white/30">
                Set in config.yaml: processing.job_history_retention,
                processing.job_history_failed_retention, processing.job_history_purge_interval.
            </p>
            <p v-if="lastRun" class="text-dim mt-2 text-[11px]">
                Last purge
                <span v-if="lastRun.trigger === 'manual'">(manual)</span>:
                <span :class="lastRun.error ? 'text-lava' : 'text-white'">
                    {{ lastRun.total.toLocaleString() }} record{{ lastRun.total === 1 ? '' : 's' }}
                </span>
                deleted,
                <NuxtTime :datetime="lastRun.started_at" relative />
                <span v-if="lastRun.error" class="text-lava"> - {{ lastRun.error }}</span>
            </p>
        </template>

        <SettingsJobsHistoryPurgeModal
            :visible="showModal"
            @close="showModal = false"
            @purged="handlePurged"
        />
    </div>
</template>
