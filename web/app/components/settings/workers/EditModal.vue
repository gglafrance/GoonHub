<script setup lang="ts">
import type { ConnectionMode, CreatedWorker, RemotePhase, RemoteWorker } from '~/types/workers';

const props = defineProps<{
    visible: boolean;
    // null = create a worker
    worker: RemoteWorker | null;
}>();

const emit = defineEmits<{
    close: [];
    // the created worker with its token, null after an edit
    saved: [created: CreatedWorker | null];
}>();

const { createWorker, updateWorker } = useApiWorkers();

const name = ref('');
const mode = ref<ConnectionMode>('pull');
const pushUrl = ref('');
const phases = ref<RemotePhase[]>([]);
const maxConcurrency = ref(0);
const loading = ref(false);
const error = ref('');

const isEdit = computed(() => props.worker !== null);

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return;
        const w = props.worker;
        name.value = w?.name ?? '';
        mode.value = w?.connection_mode ?? 'pull';
        pushUrl.value = w?.push_url ?? '';
        phases.value = w ? [...w.allowed_phases] : REMOTE_PHASES.map((p) => p.id);
        maxConcurrency.value = w?.max_concurrency ?? 0;
        error.value = '';
    },
);

function togglePhase(p: RemotePhase) {
    phases.value = phases.value.includes(p)
        ? phases.value.filter((x) => x !== p)
        : [...phases.value, p];
}

async function submit() {
    loading.value = true;
    error.value = '';
    const input = {
        name: name.value.trim(),
        connection_mode: mode.value,
        push_url: pushUrl.value.trim(),
        allowed_phases: phases.value,
        max_concurrency: Math.max(0, Math.min(64, Math.round(maxConcurrency.value || 0))),
    };
    try {
        if (props.worker) {
            await updateWorker(props.worker.id, input);
            emit('saved', null);
        } else {
            emit('saved', await createWorker(input));
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the worker';
    } finally {
        loading.value = false;
    }
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">
                    {{ isEdit ? 'Edit worker' : 'Add worker' }}
                </h3>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <form class="space-y-4" @submit.prevent="submit">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Name
                        </label>
                        <input
                            v-model="name"
                            type="text"
                            maxlength="100"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                            placeholder="e.g. gaming-pc, mac-mini"
                        />
                    </div>

                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Connection
                        </label>
                        <div class="border-border bg-void/60 inline-flex rounded-lg border p-0.5">
                            <button
                                v-for="m in ['pull', 'push'] as const"
                                :key="m"
                                type="button"
                                class="rounded-md px-3 py-1 text-[11px] font-medium transition-all"
                                :class="
                                    mode === m
                                        ? 'bg-lava/15 text-lava border-lava/30 border'
                                        : 'text-dim border border-transparent hover:text-white'
                                "
                                @click="mode = m"
                            >
                                {{
                                    m === 'pull'
                                        ? 'Pull (worker connects)'
                                        : 'Push (server assigns)'
                                }}
                            </button>
                        </div>
                        <p class="text-dim mt-1 text-[10px]">
                            The worker reports its mode when it registers; push workers also send
                            their URL.
                        </p>
                    </div>

                    <div v-if="mode === 'push'">
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Push URL
                        </label>
                        <input
                            v-model="pushUrl"
                            type="text"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 font-mono
                                text-sm text-white transition-all focus:ring-1 focus:outline-none"
                            placeholder="http://worker-1.tailnet.ts.net:9090"
                        />
                    </div>

                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Allowed phases
                        </label>
                        <div class="flex flex-wrap gap-2">
                            <button
                                v-for="p in REMOTE_PHASES"
                                :key="p.id"
                                type="button"
                                class="flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5
                                    text-[11px] transition-all"
                                :class="
                                    phases.includes(p.id)
                                        ? 'border-lava/30 bg-lava/10 text-white'
                                        : 'border-border text-dim hover:text-white'
                                "
                                @click="togglePhase(p.id)"
                            >
                                <Icon :name="p.icon" size="13" />
                                {{ p.label }}
                            </button>
                        </div>
                    </div>

                    <div class="flex items-center justify-between">
                        <div>
                            <label class="text-xs font-medium text-white">Max concurrency</label>
                            <p class="text-dim text-[10px]">
                                Jobs held at once, all phases (0 = the worker's own slots)
                            </p>
                        </div>
                        <input
                            v-model.number="maxConcurrency"
                            type="number"
                            min="0"
                            max="64"
                            class="border-border bg-surface w-16 rounded-lg border px-2 py-1.5
                                text-center text-xs text-white focus:border-white/20
                                focus:outline-none"
                        />
                    </div>

                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            @click="emit('close')"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading || !name.trim()"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            {{ loading ? 'Saving...' : isEdit ? 'Update' : 'Create' }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
