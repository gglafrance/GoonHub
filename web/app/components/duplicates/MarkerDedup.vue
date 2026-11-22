<script setup lang="ts">
import type { MarkerDedupResult } from '~/types/marker';

// Maintenance of the markers duplicated by earlier merges: a dry run over every scene shows
// what would be folded, and nothing is deleted until confirmed.
const { deduplicateMarkers } = useApiMarkers();

const visible = ref(false);
const busy = ref(false);
const error = ref<string | null>(null);
const preview = ref<MarkerDedupResult | null>(null);
const applied = ref<MarkerDedupResult | null>(null);

// Sets listed in the preview (the counts cover all of them)
const shownMerges = computed(() => preview.value?.merges.slice(0, 8) ?? []);
const moreMerges = computed(() =>
    preview.value ? preview.value.groups - shownMerges.value.length : 0,
);

const run = async (dryRun: boolean) => {
    busy.value = true;
    error.value = null;
    try {
        const result = await deduplicateMarkers({ all: true }, dryRun);
        if (dryRun) preview.value = result;
        else applied.value = result;
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Marker deduplication failed';
    } finally {
        busy.value = false;
    }
};

const open = () => {
    visible.value = true;
    preview.value = null;
    applied.value = null;
    run(true);
};

const close = () => {
    if (!busy.value) visible.value = false;
};

const handleEscape = (e: KeyboardEvent) => {
    if (visible.value && e.key === 'Escape') close();
};

onMounted(() => window.addEventListener('keydown', handleEscape));
onUnmounted(() => window.removeEventListener('keydown', handleEscape));
</script>

<template>
    <button
        class="border-border text-dim hover:border-lava/30 hover:text-lava flex items-center gap-1.5
            rounded-md border px-3 py-1.5 text-[11px] font-medium transition-all"
        title="Find markers duplicated by earlier merges"
        @click="open"
    >
        <Icon name="heroicons:bookmark-square" size="14" />
        Deduplicate markers
    </button>

    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="close"
        >
            <div
                class="border-border bg-panel mx-4 w-full max-w-md rounded-xl border p-5 shadow-2xl"
            >
                <h3 class="text-sm font-medium text-white">Deduplicate markers</h3>
                <p class="text-dim mt-1 text-xs">
                    Markers of a user with the same label (or no label) starting at most 2 seconds
                    apart are folded into one: it gets the tags of the others, which are deleted.
                </p>

                <div v-if="busy" class="flex items-center justify-center py-6">
                    <div
                        class="border-lava/30 border-t-lava h-5 w-5 animate-spin rounded-full
                            border-2"
                    ></div>
                </div>
                <p v-else-if="error" class="text-lava mt-3 text-xs">{{ error }}</p>
                <p v-else-if="applied" class="text-emerald mt-3 text-xs">
                    {{ describeMarkerDedup(applied) }}.
                </p>
                <div v-else-if="preview" class="mt-3 text-xs">
                    <p class="text-white">{{ describeMarkerDedup(preview) }}.</p>
                    <ul
                        v-if="shownMerges.length"
                        class="border-border mt-2 max-h-48 space-y-1 overflow-y-auto rounded-md
                            border bg-white/2 px-2.5 py-2 text-[11px]"
                    >
                        <li
                            v-for="m in shownMerges"
                            :key="`${m.scene_id}-${m.kept.id}`"
                            class="flex gap-2"
                        >
                            <NuxtLink
                                :to="`/watch/${m.scene_id}`"
                                class="text-lava/80 hover:text-lava shrink-0 font-mono"
                            >
                                #{{ m.scene_id }}
                            </NuxtLink>
                            <span class="text-dim">{{ describeMarkerDedupMerge(m) }}</span>
                        </li>
                        <li v-if="moreMerges > 0" class="text-dim">
                            and {{ moreMerges }} more set(s)
                        </li>
                    </ul>
                </div>

                <div class="mt-4 flex justify-end gap-2">
                    <button
                        class="border-border hover:bg-surface-hover rounded-lg border px-3 py-1.5
                            text-xs text-white transition-colors disabled:opacity-40"
                        :disabled="busy"
                        @click="close"
                    >
                        {{ applied || !preview?.removed ? 'Close' : 'Cancel' }}
                    </button>
                    <button
                        v-if="preview && preview.removed > 0 && !applied && !error"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                            font-semibold text-white transition-colors disabled:opacity-40"
                        :disabled="busy"
                        @click="run(false)"
                    >
                        Remove {{ preview.removed }} duplicate(s)
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
