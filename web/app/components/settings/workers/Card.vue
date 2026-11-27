<script setup lang="ts">
import type { CreatedWorker, RemoteWorker, WorkerInput } from '~/types/workers';

const props = defineProps<{
    worker: RemoteWorker;
}>();

const emit = defineEmits<{
    edit: [];
    changed: [];
    token: [created: CreatedWorker];
}>();

const { updateWorker, rotateWorkerToken, deleteWorker } = useApiWorkers();

const busy = ref(false);
const error = ref('');
const confirm = ref<'rotate' | 'revoke' | 'delete' | null>(null);

const status = computed(() => workerStatus(props.worker.status));
const phases = computed(() => effectivePhases(props.worker));
const caps = computed(() => props.worker.capabilities);

const machineLabel = computed(() => {
    const w = props.worker;
    const parts = [
        w.hostname,
        w.os && w.arch ? `${w.os}/${w.arch}` : w.os,
        w.version && `v${w.version}`,
    ];
    return parts.filter(Boolean).join(' · ') || 'registered';
});

async function run(fn: () => Promise<unknown>) {
    busy.value = true;
    error.value = '';
    try {
        await fn();
        emit('changed');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Request failed';
    } finally {
        busy.value = false;
    }
}

const patch = (input: WorkerInput) => run(() => updateWorker(props.worker.id, input));

async function onConfirm() {
    const action = confirm.value;
    confirm.value = null;
    if (action === 'rotate') {
        await run(async () => emit('token', await rotateWorkerToken(props.worker.id)));
    } else if (action === 'revoke') {
        await patch({ revoke_token: true });
    } else if (action === 'delete') {
        await run(() => deleteWorker(props.worker.id));
    }
}

const enabled = computed({
    get: () => props.worker.enabled,
    set: (v: boolean) => patch({ enabled: v }),
});

const draining = computed({
    get: () => props.worker.draining,
    set: (v: boolean) => patch({ draining: v }),
});

const confirmTitles = {
    rotate: 'Generate a new token?',
    revoke: 'Revoke the token?',
    delete: 'Delete this worker?',
};
</script>

<template>
    <div class="glass-panel p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0">
                <div class="flex items-center gap-2">
                    <span class="h-2 w-2 shrink-0 rounded-full" :class="status.dot" />
                    <h4 class="truncate text-sm font-semibold text-white">{{ worker.name }}</h4>
                    <span class="text-[10px] font-medium" :class="status.text">{{
                        status.label
                    }}</span>
                    <span
                        class="border-border text-dim rounded border px-1.5 py-0.5 font-mono
                            text-[10px] uppercase"
                    >
                        {{ worker.connection_mode }}
                    </span>
                    <span
                        v-if="caps.accel"
                        class="border-lava/20 bg-lava/5 text-lava rounded border px-1.5 py-0.5
                            font-mono text-[10px]"
                    >
                        {{ caps.accel }}
                    </span>
                    <span
                        v-if="worker.draining || worker.worker_draining"
                        class="rounded border border-amber-500/30 px-1.5 py-0.5 text-[10px]
                            text-amber-400"
                    >
                        draining
                    </span>
                </div>
                <p class="text-dim mt-1 font-mono text-[10px]">
                    <template v-if="worker.registered_at">
                        {{ machineLabel }}
                    </template>
                    <template v-else>not registered yet</template>
                    <template v-if="worker.last_seen_at">
                        · seen <NuxtTime :datetime="worker.last_seen_at" relative />
                    </template>
                </p>
            </div>
            <div class="flex items-center gap-4">
                <label class="text-dim flex items-center gap-2 text-[11px]">
                    Enabled
                    <UiToggle v-model="enabled" :disabled="busy" />
                </label>
                <label class="text-dim flex items-center gap-2 text-[11px]">
                    Drain
                    <UiToggle v-model="draining" :disabled="busy || !worker.enabled" />
                </label>
            </div>
        </div>

        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava mt-3 rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div class="mt-4 grid gap-4 text-[11px] sm:grid-cols-2 lg:grid-cols-4">
            <div>
                <p class="text-dim mb-1 text-[10px] tracking-wider uppercase">Phases</p>
                <div class="flex flex-wrap gap-1">
                    <span
                        v-for="p in phases"
                        :key="p"
                        class="border-border rounded border bg-white/5 px-1.5 py-0.5 text-white"
                    >
                        {{ phaseLabel(p) }}
                        <span v-if="caps.slots?.[p]" class="text-dim">×{{ caps.slots[p] }}</span>
                    </span>
                    <span v-if="phases.length === 0" class="text-dim">none</span>
                </div>
                <p v-if="worker.max_concurrency" class="text-dim mt-1 text-[10px]">
                    at most {{ worker.max_concurrency }} jobs at once
                </p>
            </div>
            <div>
                <p class="text-dim mb-1 text-[10px] tracking-wider uppercase">Input</p>
                <p class="text-white">{{ (caps.input_modes ?? []).join(', ') || '-' }}</p>
                <p
                    v-for="m in worker.path_mappings"
                    :key="m.server_prefix"
                    class="text-dim truncate font-mono text-[10px]"
                    :title="`${m.server_prefix} -> ${m.local_prefix}`"
                >
                    {{ m.server_prefix }} → {{ m.local_prefix }}
                </p>
                <p
                    v-if="!caps.audio_fingerprint && phases.includes('fingerprint')"
                    class="text-[10px] text-amber-400"
                >
                    no chromaprint: silent scenes only
                </p>
            </div>
            <div>
                <p class="text-dim mb-1 text-[10px] tracking-wider uppercase">Throughput</p>
                <p class="text-white">
                    {{ worker.jobs_completed }} done ·
                    <span :class="worker.jobs_failed ? 'text-lava' : ''"
                        >{{ worker.jobs_failed }} failed</span
                    >
                </p>
                <p class="text-dim text-[10px]">
                    {{ worker.leases_expired }} expired · {{ worker.leases_released }} released ·
                    {{ failureRate(worker) }}% lost
                </p>
            </div>
            <div>
                <p class="text-dim mb-1 text-[10px] tracking-wider uppercase">Machine</p>
                <p class="text-white">
                    {{
                        worker.free_disk_bytes ? `${formatBytes(worker.free_disk_bytes)} free` : '-'
                    }}
                </p>
                <p v-if="worker.push_url" class="text-dim truncate font-mono text-[10px]">
                    {{ worker.push_url }}
                </p>
            </div>
        </div>

        <SettingsWorkersJobList v-if="worker.jobs.length" :jobs="worker.jobs" />

        <p
            v-if="worker.last_error"
            class="text-dim mt-3 truncate text-[10px]"
            :title="worker.last_error"
        >
            Last error<template v-if="worker.last_error_at">
                (<NuxtTime :datetime="worker.last_error_at" relative />)</template
            >: <span class="text-lava/80">{{ worker.last_error }}</span>
        </p>

        <div class="border-border mt-4 flex flex-wrap justify-end gap-2 border-t pt-3">
            <button
                class="border-border rounded-lg border px-3 py-1 text-[11px] text-white
                    transition-all hover:border-white/20 hover:bg-white/5"
                :disabled="busy"
                @click="emit('edit')"
            >
                Edit
            </button>
            <button
                class="border-border rounded-lg border px-3 py-1 text-[11px] text-white
                    transition-all hover:border-white/20 hover:bg-white/5"
                :disabled="busy"
                @click="confirm = 'rotate'"
            >
                New token
            </button>
            <button
                v-if="worker.has_token"
                class="border-border rounded-lg border px-3 py-1 text-[11px] text-white
                    transition-all hover:border-white/20 hover:bg-white/5"
                :disabled="busy"
                @click="confirm = 'revoke'"
            >
                Revoke
            </button>
            <button
                class="border-lava/30 text-lava hover:bg-lava/10 rounded-lg border px-3 py-1
                    text-[11px] transition-all"
                :disabled="busy"
                @click="confirm = 'delete'"
            >
                Delete
            </button>
        </div>

        <UiConfirmModal
            :visible="confirm !== null"
            :title="confirm ? confirmTitles[confirm] : ''"
            :confirm-label="
                confirm === 'delete' ? 'Delete' : confirm === 'revoke' ? 'Revoke' : 'Generate'
            "
            @close="confirm = null"
            @confirm="onConfirm"
        >
            <p v-if="confirm === 'rotate'">
                The current token stops working at once. Its jobs go back to the queue; configure
                the worker with the new token.
            </p>
            <p v-else-if="confirm === 'revoke'">
                The worker is refused until a new token is generated. Its jobs go back to the queue.
            </p>
            <p v-else>
                The worker and its token are removed; its jobs go back to the queue. Job history is
                kept.
            </p>
        </UiConfirmModal>
    </div>
</template>
