<script setup lang="ts">
import type { QuickMarkerLabelOption } from '~/utils/quickMarker';

// Step 1 of the quick marker: the label, with the existing labels as suggestions (most used
// first). Arrows move, Tab completes, Enter confirms (empty: the last label used), Esc cancels.
const props = defineProps<{
    modelValue: string;
    labels: QuickMarkerLabelOption[];
    lastLabel: string;
    // The marker is already saved (its sub-labels failed): the label can't change anymore
    locked: boolean;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string];
    confirm: [];
    cancel: [];
}>();

const input = ref<HTMLInputElement | null>(null);
const maxLength = QUICK_MARKER_LABEL_MAX;
const highlighted = ref(-1);

const suggestions = computed(() =>
    props.locked ? [] : rankQuickMarkerLabels(props.labels, props.modelValue),
);
const rows = computed(() =>
    suggestions.value.map((s) => ({ key: s.label, text: s.label, hint: `${s.count}x` })),
);

watch(
    () => props.modelValue,
    () => {
        highlighted.value = -1;
    },
);

const setValue = (value: string) => emit('update:modelValue', value);

const pick = (index: number) => {
    const row = suggestions.value[index];
    if (!row) return;
    setValue(row.label);
    emit('confirm');
};

const onKeydown = (e: KeyboardEvent) => {
    if (e.isComposing) return;
    const count = suggestions.value.length;
    switch (e.key) {
        case 'ArrowDown':
        case 'ArrowUp':
            e.preventDefault();
            highlighted.value = stepQuickMarkerHighlight(
                highlighted.value,
                count,
                e.key === 'ArrowDown' ? 1 : -1,
                true,
            );
            break;
        case 'Tab': {
            // Completes with the highlighted (or best) suggestion; focus stays in the dialog
            e.preventDefault();
            const row = suggestions.value[Math.max(0, highlighted.value)];
            if (row && !e.shiftKey) setValue(row.label);
            break;
        }
        case 'Enter':
            e.preventDefault();
            if (highlighted.value >= 0) pick(highlighted.value);
            else emit('confirm');
            break;
        case 'Escape':
            e.preventDefault();
            emit('cancel');
            break;
    }
};

onMounted(() => nextTick(() => input.value?.focus()));
</script>

<template>
    <div>
        <label class="text-dim mb-1 block text-[10px] font-semibold tracking-wider uppercase">
            Label
        </label>
        <input
            ref="input"
            :value="modelValue"
            type="text"
            :maxlength="maxLength"
            :readonly="locked"
            :placeholder="lastLabel ? `${lastLabel} (Enter to reuse)` : 'Type a label...'"
            autocomplete="off"
            spellcheck="false"
            class="border-border bg-void/80 placeholder:text-dim focus:border-lava/50
                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-xs text-white
                focus:ring-1 focus:outline-none"
            @input="setValue(($event.target as HTMLInputElement).value)"
            @keydown="onKeydown"
        />
        <WatchQuickmarkerSuggestions
            :rows="rows"
            :highlighted="highlighted"
            @pick="pick"
            @hover="highlighted = $event"
        />
    </div>
</template>
