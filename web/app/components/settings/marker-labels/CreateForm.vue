<script setup lang="ts">
import type { MarkerLabel, MarkerLabelWithUsage } from '~/types/markerLabel';

// Create Label panel of Settings > Marker Labels.
const emit = defineEmits<{
    created: [label: MarkerLabelWithUsage];
    error: [message: string];
}>();

const { createMarkerLabel } = useApiMarkerLabels();

const name = ref('');
const description = ref('');
const color = ref<string>(TAG_COLOR_PRESETS[0]);
const creating = ref(false);

async function handleCreate() {
    if (!name.value.trim() || creating.value) return;
    creating.value = true;
    try {
        const label = await createMarkerLabel({
            name: name.value.trim(),
            color: color.value,
            description: description.value.trim(),
        });
        emit('created', { ...label, marker_count: 0, user_count: 0, default_tags: [] });
        name.value = '';
        description.value = '';
    } catch (e: unknown) {
        const conflict = getEntityConflict<MarkerLabel>(e);
        emit(
            'error',
            conflict
                ? `The label "${conflict.existing.name}" already exists`
                : e instanceof Error
                  ? e.message
                  : 'Failed to create the label',
        );
    } finally {
        creating.value = false;
    }
}
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-1 text-sm font-semibold text-white">Create Label</h3>
        <p class="text-dim mb-5 text-[11px]">
            Marker labels are shared by every user: they pick one of these when they add a marker.
        </p>
        <div class="space-y-4">
            <div class="flex flex-wrap gap-4">
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
                        placeholder="Label name"
                        :maxlength="MARKER_LABEL_MAX"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 w-64
                            rounded-lg border px-3.5 py-2.5 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                        @keydown.enter="handleCreate"
                    />
                </div>
                <div class="min-w-64 flex-1">
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Description
                    </label>
                    <input
                        v-model="description"
                        type="text"
                        placeholder="Optional"
                        maxlength="500"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-3.5 py-2.5 text-sm text-white transition-all
                            focus:ring-1 focus:outline-none"
                        @keydown.enter="handleCreate"
                    />
                </div>
            </div>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Color
                </label>
                <div class="flex items-center gap-2">
                    <button
                        v-for="preset in TAG_COLOR_PRESETS"
                        :key="preset"
                        class="h-6 w-6 rounded-full border-2 transition-all"
                        :class="
                            color === preset
                                ? 'scale-110 border-white'
                                : 'border-transparent hover:scale-110'
                        "
                        :style="{ backgroundColor: preset }"
                        @click="color = preset"
                    />
                </div>
            </div>
            <button
                :disabled="!name.trim() || creating"
                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                    text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                @click="handleCreate"
            >
                Create Label
            </button>
        </div>
    </div>
</template>
