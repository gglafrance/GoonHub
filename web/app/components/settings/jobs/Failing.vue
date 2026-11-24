<script setup lang="ts">
import type { FailingAction, FailingScene } from '~/types/jobs';
import {
    failingActionMessage,
    failingBulkBody,
    failingPhaseLabel,
    failingPhasesWithTotals,
    failingQueryString,
    failingTotal,
} from '~/utils/failingScenes';

// Failing (scene, phase): failed several times in a row on an unchanged file. The "missing"
// runs and the automatic retries skip them; Retry queues the job anyway, Clear forgets the
// failures. No action here touches a file.
const { fetchFailingScenes, retryFailingScene, clearFailingScene, bulkFailingScenes } =
    useApiJobs();

const loading = ref(true);
const actionLoading = ref<string | null>(null);
const error = ref('');
const message = ref('');

const items = ref<FailingScene[]>([]);
const totals = ref<Record<string, number>>({});
const threshold = ref(0);
const total = ref(0);
const page = ref(1);
const limit = 25;
const phase = ref('');
const search = ref('');
const selected = ref<number[]>([]);
const confirmAction = ref<FailingAction | null>(null);

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit)));
const allTotal = computed(() => failingTotal(totals.value));
const phases = computed(() => failingPhasesWithTotals(totals.value));
const bulkTarget = computed(() =>
    selected.value.length > 0 ? `${selected.value.length} selected` : `all ${total.value} shown`,
);

const load = async () => {
    loading.value = true;
    error.value = '';
    try {
        const list = await fetchFailingScenes(
            failingQueryString({ phase: phase.value, q: search.value, page: page.value, limit }),
        );
        items.value = list.data ?? [];
        totals.value = list.totals ?? {};
        threshold.value = list.threshold;
        total.value = list.total;
        if (page.value > 1 && items.value.length === 0 && list.total > 0) {
            page.value = Math.ceil(list.total / limit);
            await load();
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load failing scenes';
    } finally {
        loading.value = false;
    }
};

const showMessage = (text: string) => {
    message.value = text;
    setTimeout(() => {
        if (message.value === text) message.value = '';
    }, 4000);
};

const runRowAction = async (action: FailingAction, id: number) => {
    actionLoading.value = `${action}-${id}`;
    error.value = '';
    try {
        const res = action === 'retry' ? await retryFailingScene(id) : await clearFailingScene(id);
        selected.value = selected.value.filter((x) => x !== id);
        showMessage(failingActionMessage(action, res.result));
        await load();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : `Failed to ${action}`;
    } finally {
        actionLoading.value = null;
    }
};

const runBulkAction = async () => {
    const action = confirmAction.value;
    confirmAction.value = null;
    if (!action) return;
    actionLoading.value = `bulk-${action}`;
    error.value = '';
    try {
        const res = await bulkFailingScenes(
            failingBulkBody(action, selected.value, { phase: phase.value, q: search.value }),
        );
        selected.value = [];
        showMessage(failingActionMessage(action, res.result));
        await load();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : `Failed to ${action}`;
    } finally {
        actionLoading.value = null;
    }
};

const setPhase = (value: string) => {
    phase.value = value;
    page.value = 1;
    selected.value = [];
    load();
};

let searchTimer: ReturnType<typeof setTimeout> | undefined;
watch(search, () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
        page.value = 1;
        selected.value = [];
        load();
    }, 300);
});

const goToPage = (p: number) => {
    page.value = Math.min(Math.max(1, p), totalPages.value);
    load();
};

onMounted(load);
onBeforeUnmount(() => clearTimeout(searchTimer));
</script>

<template>
    <div class="space-y-5">
        <!-- Summary and filters -->
        <div class="glass-panel p-4">
            <div class="mb-3 flex items-center justify-between">
                <h3 class="text-sm font-semibold text-white">Failing Scenes</h3>
                <button
                    class="text-dim text-[11px] transition-colors hover:text-white"
                    @click="load"
                >
                    Refresh
                </button>
            </div>
            <p class="text-dim mb-4 text-[11px]">
                Scenes whose job failed
                <span class="text-white">{{ threshold || 'several' }}</span>
                times in a row on the same file. "Missing" runs and automatic retries skip them
                until the file changes. Retry queues the job anyway, Clear forgets the failures.
                Files are never touched.
            </p>

            <div class="mb-3 flex flex-wrap gap-2">
                <button
                    :class="[
                        'rounded-lg border px-3 py-1.5 text-[11px] transition-colors',
                        phase === ''
                            ? 'border-lava/40 bg-lava/10 text-white'
                            : 'text-dim border-white/5 bg-white/2 hover:border-white/10',
                    ]"
                    @click="setPhase('')"
                >
                    All <span class="ml-1 font-semibold">{{ allTotal }}</span>
                </button>
                <button
                    v-for="p in phases"
                    :key="p"
                    :class="[
                        'rounded-lg border px-3 py-1.5 text-[11px] transition-colors',
                        phase === p
                            ? 'border-lava/40 bg-lava/10 text-white'
                            : 'text-dim border-white/5 bg-white/2 hover:border-white/10',
                    ]"
                    @click="setPhase(p)"
                >
                    {{ failingPhaseLabel(p) }}
                    <span class="ml-1 font-semibold">{{ totals[p] }}</span>
                </button>
            </div>

            <div class="relative">
                <Icon
                    name="heroicons:magnifying-glass"
                    size="13"
                    class="text-dim absolute top-1/2 left-2.5 -translate-y-1/2"
                />
                <input
                    v-model="search"
                    type="text"
                    placeholder="Filter by error text..."
                    class="bg-surface border-border focus:border-lava/40 w-full rounded-lg border
                        py-1.5 pr-3 pl-8 text-xs text-white placeholder:text-white/30
                        focus:outline-none"
                />
            </div>
        </div>

        <!-- Alerts -->
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>
        <div
            v-if="message"
            class="rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2 text-xs
                text-emerald-400"
        >
            {{ message }}
        </div>

        <div class="glass-panel p-5">
            <!-- Bulk actions -->
            <div v-if="total > 0" class="mb-3 flex items-center justify-between gap-3">
                <span class="text-dim text-[11px]">
                    {{ total }} failing{{ phase ? ` (${failingPhaseLabel(phase)})` : '' }}
                </span>
                <div class="flex items-center gap-2">
                    <button
                        :disabled="actionLoading !== null"
                        class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-1.5
                            text-[11px] font-medium text-emerald-400 transition-colors
                            hover:bg-emerald-500/20 disabled:opacity-50"
                        @click="confirmAction = 'retry'"
                    >
                        Retry {{ bulkTarget }}
                    </button>
                    <button
                        :disabled="actionLoading !== null"
                        class="text-dim rounded-lg border border-white/10 bg-white/5 px-3 py-1.5
                            text-[11px] font-medium transition-colors hover:text-white
                            disabled:opacity-50"
                        @click="confirmAction = 'clear'"
                    >
                        Clear {{ bulkTarget }}
                    </button>
                </div>
            </div>

            <div v-if="loading && items.length === 0" class="text-dim py-8 text-center text-xs">
                Loading...
            </div>
            <div v-else-if="items.length === 0" class="text-dim py-8 text-center text-xs">
                {{
                    phase || search
                        ? 'No failing scene matches these filters'
                        : 'No failing scene: every job either succeeded or can still be retried'
                }}
            </div>
            <SettingsJobsFailingTable
                v-else
                v-model:selected="selected"
                :items="items"
                :action-loading="actionLoading"
                @retry="(id: number) => runRowAction('retry', id)"
                @clear="(id: number) => runRowAction('clear', id)"
            />

            <!-- Pagination -->
            <div
                v-if="total > limit"
                class="border-border mt-4 flex items-center justify-between border-t pt-3"
            >
                <span class="text-dim text-[11px]">{{ total }} total</span>
                <div class="flex items-center gap-3">
                    <button
                        :disabled="page <= 1"
                        class="text-dim text-[11px] transition-colors hover:text-white
                            disabled:opacity-30"
                        @click="goToPage(page - 1)"
                    >
                        Previous
                    </button>
                    <span class="text-dim text-[11px]">{{ page }} / {{ totalPages }}</span>
                    <button
                        :disabled="page >= totalPages"
                        class="text-dim text-[11px] transition-colors hover:text-white
                            disabled:opacity-30"
                        @click="goToPage(page + 1)"
                    >
                        Next
                    </button>
                </div>
            </div>
        </div>

        <UiConfirmModal
            :visible="confirmAction !== null"
            :title="confirmAction === 'retry' ? 'Retry failing scenes?' : 'Clear failures?'"
            :confirm-label="confirmAction === 'retry' ? 'Retry' : 'Clear'"
            :icon="confirmAction === 'retry' ? 'heroicons:arrow-path' : 'heroicons:x-mark'"
            @close="confirmAction = null"
            @confirm="runBulkAction"
        >
            <p v-if="confirmAction === 'retry'">
                This forgets the failures of
                <span class="text-white">{{ bulkTarget }}</span>
                and queues their job again.
            </p>
            <p v-else>
                This forgets the failures of
                <span class="text-white">{{ bulkTarget }}</span>
                without queuing anything: the next "Missing" run will include them again.
            </p>
        </UiConfirmModal>
    </div>
</template>
