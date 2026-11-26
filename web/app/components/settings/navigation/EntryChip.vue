<script setup lang="ts">
// An item of the editor: drag it to another zone, folder or Hidden, or click it to select it
// (the toolbar then offers the same moves with buttons).
const props = defineProps<{ entry: string }>();
const editor = useNavLayoutEditorContext();

const item = computed(() => NAV_ITEMS_BY_ID.get(props.entry));
const selected = computed(() => editor.selected.value === props.entry);
const dragged = computed(() => editor.dragging.value === props.entry);

function onDragstart(e: DragEvent) {
    e.stopPropagation();
    editor.dragging.value = props.entry;
    if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', props.entry);
    }
}

function onDragend() {
    editor.dragging.value = null;
    editor.over.value = null;
}
</script>

<template>
    <button
        v-if="item"
        type="button"
        draggable="true"
        class="flex cursor-grab items-center gap-1.5 rounded-md border px-2 py-1 text-[11px]
            font-medium transition-all active:cursor-grabbing"
        :class="[
            selected
                ? 'border-lava/50 bg-lava/10 text-lava'
                : 'border-border bg-panel/60 text-white/80 hover:border-white/25 hover:text-white',
            dragged ? 'opacity-40' : '',
        ]"
        :aria-pressed="selected"
        :title="item.required ? `${item.label} (always shown)` : item.label"
        @click="editor.select(selected ? null : entry)"
        @dragstart="onDragstart"
        @dragend="onDragend"
    >
        <Icon :name="item.icon" size="13" class="shrink-0" />
        <span class="whitespace-nowrap">{{ item.label }}</span>
        <Icon
            v-if="item.required"
            name="heroicons:lock-closed"
            size="10"
            class="shrink-0 text-white/30"
        />
    </button>
</template>
