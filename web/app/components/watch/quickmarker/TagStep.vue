<script setup lang="ts">
import type {
    QuickMarkerChip,
    QuickMarkerSuggestionRow,
    QuickMarkerTagOption,
} from '~/utils/quickMarker';

// Step 2 of the quick marker: the sub-labels (marker tags) as chips. Enter adds the highlighted
// suggestion or the typed tag (created on save when new and allowed); Backspace on an empty
// input removes the last chip; Enter on an empty input saves; Esc clears, then goes back.
const props = defineProps<{
    chips: QuickMarkerChip[];
    tags: QuickMarkerTagOption[];
    canCreate: boolean;
    saving: boolean;
}>();

const emit = defineEmits<{
    add: [chip: QuickMarkerChip];
    remove: [index: number];
    removeLast: [];
    submit: [];
    back: [];
}>();

const input = ref<HTMLInputElement | null>(null);
const query = ref('');
const highlighted = ref(-1);
const hint = ref<string | null>(null);
// On an empty input the suggestions only show once the arrows are used (Enter there saves)
const browsing = ref(false);

const trimmed = computed(() => query.value.trim());
const suggestions = computed(() => rankQuickMarkerTags(props.tags, query.value, props.chips));
const offerCreate = computed(() =>
    canOfferNewTag(props.tags, props.chips, query.value, props.canCreate),
);
const rows = computed<QuickMarkerSuggestionRow[]>(() => {
    if (!trimmed.value && !browsing.value) return [];
    const list: QuickMarkerSuggestionRow[] = suggestions.value.map((t) => ({
        key: `tag-${t.id}`,
        text: t.name,
        color: t.color,
        hint: t.scene_count ? String(t.scene_count) : undefined,
    }));
    if (offerCreate.value) {
        list.push({ key: 'create', text: `Create "${trimmed.value}"`, create: true });
    }
    return list;
});

// Typing highlights the best suggestion; an empty input highlights nothing (Enter saves)
watch(query, () => {
    hint.value = null;
    highlighted.value = trimmed.value && rows.value.length ? 0 : -1;
});

const added = () => {
    query.value = '';
    hint.value = null;
    browsing.value = false;
};

const pick = (index: number) => {
    const row = rows.value[index];
    if (!row) return;
    if (row.create) {
        emit('add', { id: null, name: trimmed.value });
    } else {
        const tag = suggestions.value[index];
        if (tag) emit('add', { id: tag.id, name: tag.name, color: tag.color });
    }
    added();
};

const addTyped = () => {
    const chip = chipForTypedTag(props.tags, trimmed.value, props.canCreate);
    if (!chip) {
        hint.value = `No tag "${trimmed.value}" (you can't create tags)`;
        return;
    }
    emit('add', chip);
    added();
};

const onKeydown = (e: KeyboardEvent) => {
    if (e.isComposing) return;
    switch (e.key) {
        case 'ArrowDown':
        case 'ArrowUp':
            e.preventDefault();
            browsing.value = true;
            highlighted.value = stepQuickMarkerHighlight(
                highlighted.value,
                rows.value.length,
                e.key === 'ArrowDown' ? 1 : -1,
                true,
            );
            break;
        case 'Tab': {
            e.preventDefault();
            const tag = suggestions.value[Math.max(0, highlighted.value)];
            if (tag && !e.shiftKey) query.value = tag.name;
            break;
        }
        case 'Enter':
        case ',':
            if (e.key === ',' && !trimmed.value) return;
            e.preventDefault();
            if (props.saving) return;
            if (highlighted.value >= 0) pick(highlighted.value);
            else if (trimmed.value) addTyped();
            else if (e.key === 'Enter') emit('submit');
            break;
        case 'Backspace':
            if (!query.value && props.chips.length) {
                e.preventDefault();
                emit('removeLast');
            }
            break;
        case 'Escape':
            e.preventDefault();
            if (query.value || browsing.value) added();
            else emit('back');
            break;
    }
};

onMounted(() => nextTick(() => input.value?.focus()));
</script>

<template>
    <div>
        <label class="text-dim mb-1 block text-[10px] font-semibold tracking-wider uppercase">
            Sub-labels
        </label>
        <div
            class="border-border bg-void/80 focus-within:border-lava/50 focus-within:ring-lava/20
                flex min-h-9 flex-wrap items-center gap-1 rounded-lg border px-2 py-1.5
                focus-within:ring-1"
            @mousedown.self.prevent="input?.focus()"
        >
            <span
                v-for="(chip, index) in chips"
                :key="`${chip.id ?? 'new'}-${chip.name}`"
                class="flex items-center gap-1 rounded-full border px-1.5 py-px text-[10px]
                    font-medium text-white"
                :class="chip.id == null ? 'border-lava/40 bg-lava/10' : ''"
                :style="
                    chip.id != null && chip.color
                        ? { borderColor: chip.color + '50', backgroundColor: chip.color + '15' }
                        : chip.id != null
                          ? { borderColor: 'rgba(255,255,255,0.15)' }
                          : undefined
                "
                :title="chip.id == null ? 'New tag, created on save' : undefined"
            >
                <span
                    v-if="chip.id != null"
                    class="inline-block h-1.5 w-1.5 rounded-full"
                    :style="{ backgroundColor: chip.color || '#6B7280' }"
                />
                <Icon v-else name="heroicons:sparkles" size="9" class="text-lava" />
                {{ chip.name }}
                <button
                    type="button"
                    class="text-dim hover:text-white"
                    title="Remove"
                    tabindex="-1"
                    @mousedown.prevent
                    @click="emit('remove', index)"
                >
                    <Icon name="heroicons:x-mark" size="9" />
                </button>
            </span>
            <input
                ref="input"
                v-model="query"
                type="text"
                autocomplete="off"
                spellcheck="false"
                :placeholder="chips.length ? '' : 'Add sub-labels, or Enter to save'"
                class="placeholder:text-dim min-w-24 flex-1 bg-transparent text-xs text-white
                    outline-none"
                @keydown="onKeydown"
            />
        </div>
        <p v-if="hint" class="text-dim mt-1 text-[10px]">{{ hint }}</p>
        <WatchQuickmarkerSuggestions
            :rows="rows"
            :highlighted="highlighted"
            @pick="pick"
            @hover="highlighted = $event"
        />
    </div>
</template>
