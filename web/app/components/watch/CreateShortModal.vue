<script setup lang="ts">
import type { Scene } from '~/types/scene';
import type { ShortTask } from '~/types/shorts';
import { formatShortClock, formatShortSpan } from '~/composables/useShortEditor';

const props = defineProps<{
    visible: boolean;
    scene: Scene;
    start: number | null;
    end: number | null;
    maxDuration: number;
}>();

const emit = defineEmits<{
    close: [];
    created: [task: ShortTask];
}>();

const { createShort } = useApiShorts();

const title = ref('');
const loading = ref(false);
const error = ref('');
const titleInput = ref<HTMLInputElement | null>(null);

const length = computed(() =>
    props.start !== null && props.end !== null ? props.end - props.start : 0,
);
const rangeLabel = computed(() =>
    props.start !== null && props.end !== null
        ? `${formatShortClock(props.start)}-${formatShortClock(props.end)}`
        : '',
);
const overLimit = computed(() => length.value > props.maxDuration);

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return;
        error.value = '';
        title.value = `${props.scene.title} (${rangeLabel.value})`;
        nextTick(() => titleInput.value?.select());
    },
);

const handleConfirm = async () => {
    if (props.start === null || props.end === null || loading.value) return;
    error.value = '';
    loading.value = true;
    try {
        const res = await createShort(props.scene.id, {
            start: props.start,
            end: props.end,
            title: title.value.trim(),
        });
        emit('created', res.task);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the short';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    if (loading.value) return;
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="handleClose"
            @keydown.esc="handleClose"
        >
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <h3 class="mb-1 text-sm font-semibold text-white">Save Short</h3>
                <p class="text-dim mb-4 font-mono text-[11px]">
                    {{ rangeLabel }} · {{ formatShortSpan(length) }}
                </p>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <div
                    v-if="overLimit"
                    class="mb-3 flex gap-2 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3
                        py-2 text-xs text-amber-300"
                >
                    <Icon name="heroicons:exclamation-triangle" size="14" class="mt-0.5 shrink-0" />
                    <span>
                        This clip is {{ formatShortSpan(length) }}, longer than the
                        {{ formatShortSpan(maxDuration) }} Shorts limit. It will be saved, but it
                        won't appear in the Shorts feed.
                    </span>
                </div>

                <label class="text-dim mb-1 block text-[11px] font-medium">Title</label>
                <input
                    ref="titleInput"
                    v-model="title"
                    type="text"
                    maxlength="255"
                    class="border-border bg-surface mb-4 w-full rounded-lg border px-3 py-2 text-xs
                        text-white focus:border-white/20 focus:outline-none"
                    @keydown.enter.prevent="handleConfirm"
                />

                <p class="text-dim mb-4 text-[11px]">
                    The short is encoded in the background. You can keep watching; it appears in the
                    Shorts tab when it's ready.
                </p>

                <div class="flex justify-end gap-2">
                    <button
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white"
                        :disabled="loading"
                        @click="handleClose"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading"
                        class="flex items-center gap-1.5 rounded-lg px-4 py-1.5 text-xs
                            font-semibold text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                        :class="
                            overLimit
                                ? 'bg-amber-600 hover:bg-amber-500'
                                : 'bg-lava hover:bg-lava-glow'
                        "
                        @click="handleConfirm"
                    >
                        <Icon v-if="loading" name="svg-spinners:ring-resize" size="12" />
                        {{ overLimit ? 'Save anyway' : 'Save Short' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
