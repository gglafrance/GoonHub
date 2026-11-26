<script setup lang="ts">
import type { MarkerLabel } from '~/types/markerLabel';

// Description of a marker label, saved on Enter or when leaving the field.
const props = defineProps<{
    label: MarkerLabel;
}>();

const { updateMarkerLabel } = useApiMarkerLabels();

const value = ref(props.label.description);
const saved = ref(props.label.description);
const saving = ref(false);
const error = ref('');

watch(
    () => props.label.description,
    (d) => {
        value.value = d;
        saved.value = d;
    },
);

async function save() {
    const description = value.value.trim();
    if (saving.value || description === saved.value) return;
    saving.value = true;
    error.value = '';
    try {
        const res = await updateMarkerLabel(props.label.id, { description });
        saved.value = res.description;
        value.value = res.description;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the description';
    } finally {
        saving.value = false;
    }
}
</script>

<template>
    <div>
        <label class="text-dim mb-1 block text-[10px] font-semibold tracking-wider uppercase">
            Description
        </label>
        <div class="flex items-center gap-2">
            <input
                v-model="value"
                type="text"
                maxlength="500"
                placeholder="What this label marks (optional)"
                :disabled="saving"
                class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 w-full
                    rounded-md border px-2 py-1 text-xs text-white transition-all focus:ring-1
                    focus:outline-none disabled:opacity-60"
                @keydown.enter.prevent="save"
                @blur="save"
            />
            <LoadingSpinner v-if="saving" size="sm" />
        </div>
        <p v-if="error" class="text-lava mt-1 text-[11px]">{{ error }}</p>
    </div>
</template>
