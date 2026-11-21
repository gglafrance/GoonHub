<script setup lang="ts">
import type {
    HealthAction,
    HealthCategory,
    HealthCheck,
    LibraryHealthSummary,
} from '~/types/libraryHealth';
import type { BulkJobResponse } from '~/types/jobs';

const { getSummary, getTaskStatus, startTask, cancelTask } = useApiLibraryHealth();
const { triggerBulkPhase } = useApiJobs();
const { message, error, clearMessages, setMessage, setError } = useSettingsMessage();

const TASK_POLL_INTERVAL_MS = 2000;

const loading = ref(true);
const summary = ref<LibraryHealthSummary | null>(null);
// The running background task (missing-file check or size fix), refreshed by polling
const task = ref<HealthCheck | null>(null);
const selectedCategory = ref<HealthCategory | null>(null);
const confirmAction = ref<HealthAction | null>(null);
const actionBusy = ref(false);
const sceneListRef = ref<{ reload: () => void } | null>(null);

let pollTimer: ReturnType<typeof setInterval> | null = null;

const loadSummary = async () => {
    try {
        const data = await getSummary();
        summary.value = data;
        if (data.task.running && data.task.task) {
            task.value = data.task.task;
            startPolling();
        }
    } catch (e: unknown) {
        setError(e, 'Failed to load library health');
    } finally {
        loading.value = false;
    }
};

const startPolling = () => {
    if (pollTimer) return;
    pollTimer = setInterval(pollTask, TASK_POLL_INTERVAL_MS);
};

const stopPolling = () => {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = null;
};

const pollTask = async () => {
    try {
        const status = await getTaskStatus();
        if (status.running && status.task) {
            task.value = status.task;
            return;
        }
        const finished = task.value;
        task.value = null;
        stopPolling();
        await loadSummary();
        sceneListRef.value?.reload();
        if (finished) announceFinished(finished);
    } catch (e: unknown) {
        stopPolling();
        setError(e, 'Failed to follow the task');
    }
};

const announceFinished = (finished: HealthCheck) => {
    if (finished.kind === 'missing_files') {
        const last = summary.value?.last_missing_check;
        if (last?.id === finished.id) {
            setMessage(
                `File check completed: ${last.missing} missing, ${last.unreadable} unreadable`,
            );
        } else {
            setMessage('File check stopped before the end: the previous results are kept');
        }
        return;
    }
    const last = summary.value?.last_fix_sizes;
    if (last?.id === finished.id && last.status === 'completed') {
        setMessage(`Sizes fixed: ${last.fixed} updated, ${last.missing} files missing`);
    } else {
        setMessage(`Size fix ${last?.status ?? 'stopped'}: ${last?.fixed ?? 0} sizes updated`);
    }
};

const handleView = (category: HealthCategory) => {
    selectedCategory.value = selectedCategory.value === category ? null : category;
};

const runAction = async (action: HealthAction) => {
    confirmAction.value = null;
    clearMessages();
    actionBusy.value = true;
    try {
        if (isHealthPhase(action)) {
            const result: BulkJobResponse = await triggerBulkPhase(action, 'missing');
            let text = `${result.submitted} ${healthPhaseLabel(action)} jobs queued`;
            if (result.errors) text += `, ${result.errors} failed`;
            setMessage(text);
            await loadSummary();
        } else {
            task.value = await startTask(action);
            startPolling();
        }
    } catch (e: unknown) {
        setError(e, 'Action failed');
    } finally {
        actionBusy.value = false;
    }
};

const handleCancel = async () => {
    clearMessages();
    try {
        await cancelTask();
    } catch (e: unknown) {
        setError(e, 'Failed to cancel the task');
    }
};

const categoryCountFor = (action: HealthAction | null): number | undefined => {
    if (!action || !summary.value) return undefined;
    const info = HEALTH_GROUPS.flatMap((g) => g.categories).find((c) => c.action === action);
    return info ? summary.value.counts[info.key] : summary.value.total_scenes;
};

onMounted(loadSummary);
onUnmounted(stopPolling);
</script>

<template>
    <div class="space-y-5">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

        <template v-else-if="summary">
            <SettingsHealthFileCheck
                :summary="summary"
                :task="task"
                :busy="actionBusy"
                @start="confirmAction = 'missing_files'"
                @cancel="handleCancel"
                @refresh="loadSummary"
            />

            <SettingsHealthCategories
                :summary="summary"
                :selected="selectedCategory"
                :task-running="task !== null"
                :busy="actionBusy"
                @view="handleView"
                @action="confirmAction = $event"
            />

            <SettingsHealthSceneList
                v-if="selectedCategory"
                ref="sceneListRef"
                :key="selectedCategory"
                :category="selectedCategory"
                @close="selectedCategory = null"
            />

            <SettingsHealthStoragePaths :paths="summary.storage_paths" />
        </template>

        <SettingsHealthActionConfirm
            :action="confirmAction"
            :category-count="categoryCountFor(confirmAction)"
            @close="confirmAction = null"
            @confirm="runAction"
        />
    </div>
</template>
