<script setup lang="ts">
import type { CreatedWorker, RemoteWorker, WorkersListResponse } from '~/types/workers';

const { fetchWorkers } = useApiWorkers();

const state = ref<WorkersListResponse | null>(null);
const loading = ref(true);
const error = ref('');
const showCreate = ref(false);
const editing = ref<RemoteWorker | null>(null);
const created = ref<CreatedWorker | null>(null);

let timer: ReturnType<typeof setInterval> | null = null;

async function load(silent = false) {
    if (!silent) loading.value = true;
    try {
        state.value = await fetchWorkers();
        error.value = '';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load workers';
    } finally {
        loading.value = false;
    }
}

function onSaved(result: CreatedWorker | null) {
    showCreate.value = false;
    editing.value = null;
    if (result) created.value = result;
    load(true);
}

function onToken(result: CreatedWorker) {
    created.value = result;
    load(true);
}

onMounted(() => {
    load();
    // Status, last seen and job progress follow the heartbeats
    timer = setInterval(() => load(true), 5000);
});

onUnmounted(() => {
    if (timer) clearInterval(timer);
});
</script>

<template>
    <div class="space-y-4">
        <div class="glass-panel p-5">
            <div class="flex items-start justify-between gap-4">
                <div>
                    <h3 class="flex items-center gap-2 text-sm font-semibold text-white">
                        Remote workers
                        <span
                            v-if="state"
                            class="rounded-full border px-2 py-0.5 text-[10px] font-medium"
                            :class="
                                state.enabled
                                    ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
                                    : 'border-border text-dim'
                            "
                        >
                            {{ state.enabled ? 'API enabled' : 'API disabled' }}
                        </span>
                    </h3>
                    <p class="text-dim mt-1 max-w-2xl text-[11px]">
                        Machines running goonhub-worker process sprites, animated thumbnails and
                        fingerprints with their own hardware. A stopped or lost worker never loses a
                        job: its jobs go back to the queue.
                    </p>
                </div>
                <button
                    class="bg-lava hover:bg-lava-glow flex shrink-0 items-center gap-1.5 rounded-lg
                        px-3 py-1.5 text-[11px] font-semibold text-white transition-all"
                    @click="showCreate = true"
                >
                    <Icon name="heroicons:plus" size="14" />
                    Add worker
                </button>
            </div>

            <div
                v-if="state && !state.enabled"
                class="mt-4 rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2
                    text-[11px] text-amber-300"
            >
                The worker API is off (<code class="font-mono">workers.enabled: false</code> in the
                server config): workers cannot connect and every phase runs on the server. Workers
                and tokens can still be prepared here.
            </div>

            <div
                v-if="error"
                class="border-lava/20 bg-lava/5 text-lava mt-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ error }}
            </div>

            <div
                v-if="state"
                class="text-dim mt-4 flex flex-wrap gap-x-5 gap-y-1 font-mono text-[10px]"
            >
                <span>heartbeat {{ state.settings.heartbeat_interval_sec }}s</span>
                <span>lease {{ state.settings.lease_duration_sec }}s</span>
                <span>offline after {{ state.settings.offline_after_sec }}s</span>
                <span v-if="state.settings.public_url"
                    >media via {{ state.settings.public_url }}</span
                >
            </div>
        </div>

        <SettingsWorkersRouting
            v-if="state"
            :routing="state.routing"
            :workers="state.workers"
            @updated="load(true)"
        />

        <div v-if="loading && !state" class="text-dim py-8 text-center text-xs">Loading...</div>

        <div
            v-else-if="state && state.workers.length === 0"
            class="glass-panel text-dim flex flex-col items-center gap-2 p-8 text-center text-xs"
        >
            <Icon name="heroicons:server-stack" size="28" class="text-white/20" />
            <p>No worker yet. Add one, then start goonhub-worker with its token.</p>
        </div>

        <SettingsWorkersCard
            v-for="w in state?.workers ?? []"
            :key="w.id"
            :worker="w"
            @edit="editing = w"
            @changed="load(true)"
            @token="onToken"
        />

        <SettingsWorkersEditModal
            :visible="showCreate || editing !== null"
            :worker="editing"
            @close="
                showCreate = false;
                editing = null;
            "
            @saved="onSaved"
        />

        <SettingsWorkersTokenModal
            :visible="created !== null"
            :created="created"
            @close="created = null"
        />
    </div>
</template>
