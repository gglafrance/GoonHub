<script setup lang="ts">
import type { TriageOption } from '~/types/triage';

// Keyboard-first field of the triage page: current values as chips plus an autocomplete
// input (arrows to move, Enter to pick or create, Esc to go back to the triage keys).

const props = defineProps<{
    label: string;
    icon: string;
    shortcut: string;
    items: { id: number; name: string }[];
    canEdit: boolean;
    allowCreate: boolean;
    search: (query: string) => Promise<TriageOption[]>;
    placeholder?: string;
    // One value at most (studio): picking replaces it
    single?: boolean;
    // Link of a chip, if any
    linkFor?: (item: { id: number; name: string }) => string | undefined;
}>();

const emit = defineEmits<{
    add: [option: TriageOption];
    create: [name: string];
    remove: [id: number];
    exit: [];
}>();

const input = ref<HTMLInputElement | null>(null);
const query = ref('');
const focused = ref(false);
const options = ref<TriageOption[]>([]);
const highlighted = ref(0);
const searching = ref(false);
// Enter on an empty input picks a suggestion only after the user moved with the arrows
const focusedByArrows = ref(false);
let seq = 0;
let debounce: ReturnType<typeof setTimeout> | null = null;

const trimmed = computed(() => query.value.trim());

const visibleOptions = computed(() => {
    const taken = new Set(props.items.map((i) => i.id));
    return options.value.filter((o) => !taken.has(o.id)).slice(0, 8);
});

const showCreate = computed(() => {
    if (!props.allowCreate || !trimmed.value) return false;
    const q = trimmed.value.toLowerCase();
    return ![...options.value, ...props.items].some((o) => o.name.toLowerCase() === q);
});

// Rows: options then the "create" row
const rowCount = computed(() => visibleOptions.value.length + (showCreate.value ? 1 : 0));

const runSearch = () => {
    if (debounce) clearTimeout(debounce);
    debounce = setTimeout(async () => {
        const mine = ++seq;
        searching.value = true;
        try {
            const result = await props.search(trimmed.value);
            if (mine === seq) {
                options.value = result;
                highlighted.value = 0;
            }
        } catch {
            if (mine === seq) options.value = [];
        } finally {
            if (mine === seq) searching.value = false;
        }
    }, 120);
};

watch(query, () => {
    focusedByArrows.value = false;
    runSearch();
});

const onFocus = () => {
    focused.value = true;
    runSearch();
};

const onBlur = () => {
    focused.value = false;
    focusedByArrows.value = false;
};

const pick = (row: number) => {
    const option = visibleOptions.value[row];
    if (option) {
        emit('add', option);
    } else if (showCreate.value) {
        emit('create', trimmed.value);
    } else {
        return;
    }
    query.value = '';
    if (props.single) exit();
};

const exit = () => {
    query.value = '';
    input.value?.blur();
    emit('exit');
};

const onKeydown = (e: KeyboardEvent) => {
    switch (e.key) {
        case 'ArrowDown':
            e.preventDefault();
            if (rowCount.value > 0) highlighted.value = (highlighted.value + 1) % rowCount.value;
            break;
        case 'ArrowUp':
            e.preventDefault();
            if (rowCount.value > 0)
                highlighted.value = (highlighted.value - 1 + rowCount.value) % rowCount.value;
            break;
        case 'Enter':
            e.preventDefault();
            // Empty input: Enter goes back to the triage keys, unless a suggestion was chosen
            // with the arrows
            if (!trimmed.value && !focusedByArrows.value) exit();
            else pick(highlighted.value);
            break;
        case 'Escape':
            e.preventDefault();
            if (query.value) query.value = '';
            else exit();
            break;
    }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') focusedByArrows.value = true;
};

defineExpose({
    focus: () => input.value?.focus(),
});
</script>

<template>
    <div class="space-y-1.5">
        <div class="flex items-center gap-1.5">
            <Icon :name="icon" size="12" class="text-dim" />
            <h3 class="text-dim text-[10px] font-semibold tracking-wider uppercase">
                {{ label }}
            </h3>
            <kbd
                v-if="canEdit"
                class="border-border bg-panel text-dim ml-auto rounded border px-1 font-mono
                    text-[9px]"
                >{{ shortcut }}</kbd
            >
        </div>

        <div class="flex flex-wrap items-center gap-1">
            <span
                v-for="item in items"
                :key="item.id"
                class="border-border bg-panel group inline-flex max-w-full items-center gap-1
                    rounded-full border py-0.5 pr-1 pl-2 text-[11px] text-white"
            >
                <NuxtLink
                    v-if="linkFor && linkFor(item)"
                    :to="linkFor(item)!"
                    target="_blank"
                    class="hover:text-lava truncate"
                    >{{ item.name }}</NuxtLink
                >
                <span v-else class="truncate">{{ item.name }}</span>
                <button
                    v-if="canEdit"
                    class="text-dim hover:text-lava rounded-full p-0.5 transition-colors"
                    :title="`Remove ${item.name}`"
                    tabindex="-1"
                    @click="emit('remove', item.id)"
                >
                    <Icon name="heroicons:x-mark" size="10" />
                </button>
            </span>
            <span v-if="items.length === 0" class="text-dim text-[11px] italic">None</span>
        </div>

        <div v-if="canEdit" class="relative">
            <input
                ref="input"
                v-model="query"
                type="text"
                :placeholder="placeholder"
                class="border-border bg-surface placeholder:text-dim focus:border-lava/50 h-8 w-full
                    rounded-md border px-2.5 text-xs text-white transition-colors
                    focus:outline-none"
                autocomplete="off"
                @focus="onFocus"
                @blur="onBlur"
                @keydown="onKeydown"
            />
            <LoadingSpinner
                v-if="searching && focused"
                class="absolute top-1/2 right-2 -translate-y-1/2 scale-75"
            />

            <div
                v-if="focused && rowCount > 0"
                class="border-border bg-panel/95 absolute top-full right-0 left-0 z-30 mt-1
                    overflow-hidden rounded-md border shadow-2xl backdrop-blur-md"
            >
                <button
                    v-for="(option, i) in visibleOptions"
                    :key="option.id"
                    class="flex w-full items-center justify-between gap-2 px-2.5 py-1.5 text-left
                        text-xs transition-colors"
                    :class="i === highlighted ? 'bg-lava/15 text-white' : 'text-muted'"
                    tabindex="-1"
                    @mousedown.prevent="pick(i)"
                    @mouseenter="highlighted = i"
                >
                    <span class="truncate">{{ option.name }}</span>
                    <span v-if="option.hint" class="text-dim shrink-0 font-mono text-[10px]">{{
                        option.hint
                    }}</span>
                </button>
                <button
                    v-if="showCreate"
                    class="border-border flex w-full items-center gap-1.5 border-t px-2.5 py-1.5
                        text-left text-xs transition-colors"
                    :class="
                        highlighted === visibleOptions.length
                            ? 'bg-lava/15 text-white'
                            : 'text-muted'
                    "
                    tabindex="-1"
                    @mousedown.prevent="pick(visibleOptions.length)"
                    @mouseenter="highlighted = visibleOptions.length"
                >
                    <Icon name="heroicons:plus" size="12" class="text-lava" />
                    Create "{{ trimmed }}"
                </button>
            </div>
        </div>
    </div>
</template>
