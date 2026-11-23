<script setup lang="ts">
import type { RemotePhase, RemoteWorker, RoutingMode, WorkerRouting } from '~/types/workers';

const props = defineProps<{
    routing: WorkerRouting;
    workers: RemoteWorker[];
}>();

const emit = defineEmits<{
    updated: [];
}>();

const { updateWorkerRouting } = useApiWorkers();

const saving = ref<RemotePhase | null>(null);
const error = ref('');

const unserved = computed(() => unservedRemotePhases(props.routing.effective, props.workers));

async function setMode(phase: RemotePhase, mode: RoutingMode) {
    if (props.routing.effective[phase] === mode) return;
    saving.value = phase;
    error.value = '';
    try {
        const overrides: Record<string, RoutingMode | ''> = { ...props.routing.overrides };
        // Back to the config default: drop the override
        overrides[phase] = props.routing.defaults[phase] === mode ? '' : mode;
        await updateWorkerRouting(overrides);
        emit('updated');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update the routing';
    } finally {
        saving.value = null;
    }
}
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4">
            <h3 class="text-sm font-semibold text-white">Phase routing</h3>
            <p class="text-dim mt-1 text-[11px]">
                Who processes each phase. Remote means the server pool stops claiming (like 0 local
                workers); metadata and thumbnails always run on the server.
            </p>
        </div>

        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div class="space-y-3">
            <div
                v-for="phase in REMOTE_PHASES"
                :key="phase.id"
                class="flex flex-wrap items-center justify-between gap-3"
            >
                <div>
                    <label class="flex items-center gap-1.5 text-xs font-medium text-white">
                        <Icon :name="phase.icon" size="13" class="text-dim" />
                        {{ phase.label }}
                        <span
                            v-if="routing.overrides[phase.id] === undefined"
                            class="text-dim text-[10px] font-normal"
                        >
                            (config default)
                        </span>
                    </label>
                    <p v-if="unserved.includes(phase.id)" class="mt-0.5 text-[10px] text-amber-400">
                        No online worker takes this phase: its jobs wait in the queue.
                    </p>
                </div>
                <div class="border-border bg-void/60 flex rounded-lg border p-0.5">
                    <button
                        v-for="mode in ROUTING_MODES"
                        :key="mode.id"
                        type="button"
                        :title="mode.hint"
                        :disabled="!routing.enabled || saving !== null"
                        class="rounded-md px-3 py-1 text-[11px] font-medium transition-all
                            disabled:cursor-not-allowed"
                        :class="
                            routing.effective[phase.id] === mode.id
                                ? 'bg-lava/15 text-lava border-lava/30 border'
                                : 'text-dim border border-transparent hover:text-white'
                        "
                        @click="setMode(phase.id, mode.id)"
                    >
                        {{ mode.label }}
                    </button>
                </div>
            </div>
        </div>
        <p v-if="!routing.enabled" class="text-dim mt-3 text-[10px]">
            The routing applies once the worker API is enabled.
        </p>
    </div>
</template>
