<script setup lang="ts">
import type { NavLocation } from '~/utils/navLayout';

// A drop area of the editor (a zone, the items of a folder, or Hidden): its entries in order,
// and while dragging, a marker where the dragged entry would go. A list inside another (a
// folder in a zone) handles its own drops; an entry it cannot take (a folder) falls through to
// the zone around it.
const props = defineProps<{
    loc: NavLocation;
    entries: string[];
    label: string;
    emptyLabel?: string;
}>();

const editor = useNavLayoutEditorContext();
const key = computed(() => navLocationKey(props.loc));
const overIndex = computed(() =>
    editor.over.value?.key === key.value ? editor.over.value.index : null,
);
const accepting = computed(
    () => !!editor.dragging.value && editor.canPlace(editor.dragging.value, props.loc),
);

/** Index where the pointer would insert, from the chips of this list (rows may wrap). */
function indexAt(e: DragEvent): number {
    const el = e.currentTarget as HTMLElement;
    const chips = Array.from(el.querySelectorAll<HTMLElement>(':scope > [data-nav-entry]'));
    for (let i = 0; i < chips.length; i++) {
        const r = chips[i]!.getBoundingClientRect();
        if (e.clientY < r.top) return i;
        if (e.clientY <= r.bottom && e.clientX < r.left + r.width / 2) return i;
    }
    return chips.length;
}

function onDragover(e: DragEvent) {
    if (!accepting.value) return;
    e.preventDefault();
    e.stopPropagation();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    const index = indexAt(e);
    if (overIndex.value !== index) editor.over.value = { key: key.value, index };
}

function onDragleave(e: DragEvent) {
    const el = e.currentTarget as HTMLElement;
    if (el.contains(e.relatedTarget as Node | null)) return;
    if (editor.over.value?.key === key.value) editor.over.value = null;
}

function onDrop(e: DragEvent) {
    const entry = editor.dragging.value;
    if (!entry || !accepting.value) return;
    e.preventDefault();
    e.stopPropagation();
    let index = indexAt(e);
    // the entry leaves its place first when it moves within this list
    const from = findNavEntry(editor.draft.value, entry);
    if (from && navLocationKey(from.loc) === key.value && from.index < index) index--;
    editor.move(entry, props.loc, index);
    editor.dragging.value = null;
    editor.over.value = null;
    editor.select(entry);
}
</script>

<template>
    <div
        role="group"
        :aria-label="label"
        class="flex min-h-10 flex-wrap items-center gap-1.5 rounded-lg border border-dashed p-1.5
            transition-colors"
        :class="
            accepting
                ? overIndex !== null
                    ? 'border-lava/50 bg-lava/5'
                    : 'border-white/20 bg-white/[0.02]'
                : 'border-white/10'
        "
        @dragover="onDragover"
        @dragleave="onDragleave"
        @drop="onDrop"
    >
        <template v-for="(entry, i) in entries" :key="entry">
            <span
                v-if="overIndex === i"
                class="bg-lava h-6 w-0.5 rounded-full shadow-[0_0_6px_rgba(255,77,77,0.6)]"
            />
            <SettingsNavigationFolderBlock
                v-if="navFolderRefId(entry) !== null"
                :entry="entry"
                data-nav-entry
            />
            <SettingsNavigationEntryChip v-else :entry="entry" data-nav-entry />
        </template>
        <span
            v-if="overIndex === entries.length"
            class="bg-lava h-6 w-0.5 rounded-full shadow-[0_0_6px_rgba(255,77,77,0.6)]"
        />
        <span
            v-if="entries.length === 0 && overIndex === null"
            class="px-1 text-[10px] text-white/30"
        >
            {{ emptyLabel ?? 'Empty' }}
        </span>
    </div>
</template>
