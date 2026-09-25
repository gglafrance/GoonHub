<script setup lang="ts">
import type { ShortEditor } from '~/composables/useShortEditor';
import { SHORT_EDITOR_KEY, formatShortClock, formatShortSpan } from '~/composables/useShortEditor';

// Header of the Shorts tab: "+ Add short", or the short-mode panel with the
// live A/B readout and the Save Short button.
const editor = inject<ShortEditor>(SHORT_EDITOR_KEY);
const seekToTime = inject<(time: number) => void>('seekToTime');

const active = computed(() => editor?.active.value ?? false);
const canCreate = computed(() => editor?.canCreate.value ?? false);
const pointA = computed(() => editor?.pointA.value ?? null);
const pointB = computed(() => editor?.pointB.value ?? null);
const length = computed(() => editor?.length.value ?? null);
const maxDuration = computed(() => editor?.maxDuration.value ?? 60);
const state = computed(
    () => editor?.saveState.value ?? { valid: false, reason: '', warning: null },
);

const seek = (t: number | null) => {
    if (t !== null && seekToTime) seekToTime(t);
};
</script>

<template>
    <div v-if="editor && canCreate">
        <div v-if="!active" class="flex items-center justify-end">
            <button
                class="bg-lava hover:bg-lava/80 flex items-center gap-1.5 rounded-lg px-3 py-2
                    text-xs font-medium text-white transition-colors"
                title="Cut a short from this scene with the player's A/B controls"
                @click="editor.start()"
            >
                <Icon name="heroicons:plus" size="12" />
                Add short
            </button>
        </div>

        <div v-else class="border-lava/25 bg-lava/5 space-y-3 rounded-lg border p-3">
            <p class="text-dim text-xs leading-relaxed">
                Use the A/B controls in the player to choose where your short starts and ends: click
                <strong class="text-white">A</strong> at the start and
                <strong class="text-white">B</strong> at the end (or press
                <kbd class="rounded bg-white/10 px-1 font-mono text-[10px] text-white">O</kbd>
                once for each). When the range is right, click
                <strong class="text-white">Save Short</strong>. Shorts can be up to
                {{ formatShortSpan(maxDuration) }} long.
            </p>

            <div class="flex flex-wrap items-center gap-2">
                <div class="flex items-center gap-1.5 font-mono text-[11px]">
                    <button
                        class="border-border rounded-md border px-2 py-1 transition-colors"
                        :class="
                            pointA !== null
                                ? 'hover:border-lava/40 text-white'
                                : 'text-dim cursor-default'
                        "
                        :title="pointA !== null ? 'Go to the start point' : 'No start point yet'"
                        @click="seek(pointA)"
                    >
                        <span class="text-lava mr-1 font-bold">A</span>
                        {{ pointA !== null ? formatShortClock(pointA) : '--:--' }}
                    </button>
                    <span class="text-dim">·</span>
                    <button
                        class="border-border rounded-md border px-2 py-1 transition-colors"
                        :class="
                            pointB !== null
                                ? 'hover:border-lava/40 text-white'
                                : 'text-dim cursor-default'
                        "
                        :title="pointB !== null ? 'Go to the end point' : 'No end point yet'"
                        @click="seek(pointB)"
                    >
                        <span class="text-lava mr-1 font-bold">B</span>
                        {{ pointB !== null ? formatShortClock(pointB) : '--:--' }}
                    </button>
                    <span class="text-dim">·</span>
                    <span :class="state.warning ? 'text-amber-300' : 'text-white'">
                        {{ length !== null ? formatShortSpan(length) : '-:--' }}
                    </span>
                </div>

                <div class="ml-auto flex items-center gap-2">
                    <button
                        class="text-dim rounded-lg px-3 py-2 text-xs transition-colors
                            hover:text-white"
                        @click="editor.cancel()"
                    >
                        Cancel
                    </button>
                    <button
                        class="flex items-center gap-1.5 rounded-lg px-3 py-2 text-xs font-medium
                            transition-colors"
                        :class="[
                            !state.valid
                                ? 'cursor-not-allowed bg-white/5 text-white/35'
                                : state.warning
                                  ? 'bg-amber-600 text-white hover:bg-amber-500'
                                  : 'bg-lava hover:bg-lava/80 text-white',
                        ]"
                        :aria-disabled="!state.valid"
                        :title="state.warning ?? state.reason"
                        @click="editor.save()"
                    >
                        <Icon name="heroicons:bookmark-square" size="12" />
                        Save Short
                    </button>
                </div>
            </div>

            <div
                v-if="state.warning"
                class="flex gap-2 rounded-md border border-amber-500/30 bg-amber-500/5 px-2.5 py-1.5
                    text-[11px] text-amber-300"
            >
                <Icon name="heroicons:exclamation-triangle" size="13" class="mt-px shrink-0" />
                {{ state.warning }}
            </div>
            <p v-else-if="!state.valid" class="text-dim text-[11px]">{{ state.reason }}</p>
        </div>
    </div>
</template>
