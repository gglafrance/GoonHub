<script setup lang="ts">
import type { MarkerTimeField, MarkerTimeRange } from '~/utils/markerTime';

// Typed start or end of a marker card: ss, mm:ss or h:mm:ss, with -1s/+1s nudges (also
// ArrowDown/ArrowUp). Enter saves, Escape cancels. The time is checked like the API does before
// it is sent; the API error is shown when the save fails. The end can be removed (point marker):
// "No end" button, or an empty field and Enter.
const props = defineProps<{
    field: MarkerTimeField;
    marker: MarkerTimeRange;
    // Scene duration in seconds (unknown: 0 or null)
    duration?: number | null;
    saving?: boolean;
    // Error of the last save (shown until the time is changed)
    serverError?: string | null;
}>();

const emit = defineEmits<{
    save: [seconds: number];
    // Remove the end of the marker (end editor only)
    clear: [];
    cancel: [];
}>();

const current = computed(() =>
    props.field === 'start' ? props.marker.timestamp : (props.marker.end_timestamp ?? 0),
);

const draft = ref(formatMarkerTime(current.value));
// Text that was sent: the API error belongs to it
const submitted = ref<string | null>(null);
const inputRef = ref<HTMLInputElement | null>(null);

const value = computed(() => parseMarkerTime(draft.value));
// Empty end field: Enter (or the check button) removes the end
const clearing = computed(() => isMarkerEndClear(props.field, draft.value));
const invalid = computed(() =>
    clearing.value
        ? null
        : validateMarkerTimeEdit(props.field, value.value, props.marker, props.duration),
);
const message = computed(
    () => invalid.value ?? (submitted.value === draft.value ? props.serverError : null),
);

const focusInput = () =>
    nextTick(() => {
        inputRef.value?.focus();
        inputRef.value?.select();
    });

const nudge = (delta: number) => {
    draft.value = nudgeMarkerTime(draft.value, delta, current.value, props.duration);
};

const clearEnd = () => {
    if (props.saving || props.field !== 'end') return;
    submitted.value = draft.value;
    emit('clear');
};

const save = () => {
    if (clearing.value) {
        clearEnd();
        return;
    }
    if (props.saving || invalid.value || value.value == null) return;
    submitted.value = draft.value;
    emit('save', value.value);
};

const onKeydown = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
        e.preventDefault();
        e.stopPropagation();
        save();
    } else if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        emit('cancel');
    } else if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        e.preventDefault();
        e.stopPropagation();
        nudge(e.key === 'ArrowUp' ? 1 : -1);
    }
};

// The card may move in the list (kept in start order) while the time is saved: take the focus
// back when the save fails
watch(
    () => props.serverError,
    (err) => {
        if (err) focusInput();
    },
);

onMounted(focusInput);
</script>

<template>
    <div class="border-border bg-panel/60 space-y-1 border-b px-2 py-1.5" @click.stop>
        <div class="text-dim flex items-center justify-between text-[10px]">
            <span class="font-medium text-white">{{ field === 'start' ? 'Start' : 'End' }}</span>
            <span>{{
                field === 'end' ? 'ss, mm:ss or h:mm:ss, empty: no end' : 'ss, mm:ss or h:mm:ss'
            }}</span>
        </div>
        <div class="flex items-center gap-1">
            <button
                type="button"
                class="text-dim hover:bg-surface-hover shrink-0 rounded px-1 py-0.5 font-mono
                    text-[10px] transition-colors hover:text-white"
                title="1 second earlier (ArrowDown)"
                @mousedown.prevent
                @click="nudge(-1)"
            >
                -1s
            </button>
            <input
                ref="inputRef"
                v-model="draft"
                type="text"
                inputmode="numeric"
                maxlength="10"
                :aria-label="field === 'start' ? 'Marker start' : 'Marker end'"
                :aria-invalid="!!message"
                class="bg-surface w-full min-w-0 rounded-md border px-1.5 py-0.5 text-center
                    font-mono text-[11px] text-white focus:outline-none"
                :class="message ? 'border-lava/60' : 'border-border focus:border-white/20'"
                @keydown="onKeydown"
            />
            <button
                type="button"
                class="text-dim hover:bg-surface-hover shrink-0 rounded px-1 py-0.5 font-mono
                    text-[10px] transition-colors hover:text-white"
                title="1 second later (ArrowUp)"
                @mousedown.prevent
                @click="nudge(1)"
            >
                +1s
            </button>
            <button
                type="button"
                :disabled="saving || !!invalid"
                class="text-emerald hover:bg-emerald/10 shrink-0 rounded p-1 transition-colors
                    disabled:cursor-not-allowed disabled:opacity-40"
                :title="clearing ? 'Remove the end (Enter)' : 'Save (Enter)'"
                @mousedown.prevent
                @click="save"
            >
                <Icon v-if="saving" name="svg-spinners:ring-resize" size="12" />
                <Icon v-else name="heroicons:check" size="12" />
            </button>
            <button
                v-if="field === 'end'"
                type="button"
                :disabled="saving"
                class="text-dim hover:text-lava hover:bg-lava/10 shrink-0 rounded px-1 py-0.5
                    text-[10px] whitespace-nowrap transition-colors disabled:cursor-not-allowed
                    disabled:opacity-40"
                title="Remove the end: back to a point marker (or empty the field and press Enter)"
                @mousedown.prevent
                @click="clearEnd"
            >
                No end
            </button>
            <button
                type="button"
                class="text-dim hover:bg-surface-hover shrink-0 rounded p-1 transition-colors"
                title="Cancel (Esc)"
                @mousedown.prevent
                @click="emit('cancel')"
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </div>
        <p v-if="message" class="text-[10px] leading-tight text-red-300" role="alert">
            {{ message }}
        </p>
    </div>
</template>
