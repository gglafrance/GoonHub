<script setup lang="ts">
import type { CreatedWorker } from '~/types/workers';

const props = defineProps<{
    visible: boolean;
    created: CreatedWorker | null;
}>();

const emit = defineEmits<{
    close: [];
}>();

const { copied, manualText, copy: copyToClipboard, dismiss } = useClipboard();

watch(
    () => props.visible,
    () => {
        copied.value = false;
        dismiss();
    },
);

function copy() {
    if (props.created) copyToClipboard(props.created.token);
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible && created"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
        >
            <div class="glass-panel border-border w-full max-w-lg border p-6">
                <h3 class="mb-1 text-sm font-semibold text-white">
                    Token of {{ created.worker.name }}
                </h3>
                <p class="text-dim mb-4 text-[11px]">
                    Shown only now: the server keeps a hash of it. Put it in the worker config
                    (<code class="font-mono">token</code>) or its environment.
                </p>
                <div
                    class="border-border bg-void/80 rounded-lg border px-3 py-2.5 font-mono text-xs
                        break-all text-white select-all"
                >
                    {{ created.token }}
                </div>
                <p v-if="manualText" class="mt-2 text-[11px] text-amber-400">
                    The browser blocked copying here: select the token above and copy it by hand.
                </p>

                <div class="mt-4 flex justify-end gap-2">
                    <button
                        class="border-border rounded-lg border px-3 py-1.5 text-xs text-white
                            transition-all hover:border-white/20 hover:bg-white/5"
                        @click="copy"
                    >
                        <Icon
                            :name="copied ? 'heroicons:check' : 'heroicons:clipboard-document'"
                            size="13"
                            class="mr-1 inline"
                        />
                        {{ copied ? 'Copied' : 'Copy' }}
                    </button>
                    <button
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                            font-semibold text-white transition-all"
                        @click="emit('close')"
                    >
                        Done
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
