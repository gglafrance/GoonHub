<script setup lang="ts">
// A folder in the editor: its button (drag it to another zone, click it to rename, change its
// icon or dissolve it) and the list of its items (a drop area for the items that can go in a
// folder).
const props = defineProps<{ entry: string }>();
const editor = useNavLayoutEditorContext();

const folderId = computed(() => navFolderRefId(props.entry) ?? '');
const folder = computed(() => editor.draft.value.folders.find((f) => f.id === folderId.value));
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
    <div
        v-if="folder"
        class="flex min-w-40 flex-col gap-1 rounded-lg border p-1 transition-all"
        :class="[
            selected ? 'border-lava/50 bg-lava/5' : 'border-white/15 bg-white/[0.02]',
            dragged ? 'opacity-40' : '',
        ]"
    >
        <button
            type="button"
            draggable="true"
            class="flex cursor-grab items-center gap-1.5 rounded-md px-1.5 py-0.5 text-[11px]
                font-semibold transition-colors active:cursor-grabbing"
            :class="selected ? 'text-lava' : 'hover:text-lava text-white'"
            :aria-pressed="selected"
            :title="`Folder ${folder.name}`"
            @click="editor.select(selected ? null : entry)"
            @dragstart="onDragstart"
            @dragend="onDragend"
        >
            <Icon :name="folder.icon || 'heroicons:folder'" size="13" class="shrink-0" />
            <span class="truncate">{{ folder.name }}</span>
            <Icon name="heroicons:chevron-down" size="10" class="shrink-0 text-white/40" />
        </button>
        <SettingsNavigationEntryList
            :loc="{ kind: 'folder', folderId: folder.id }"
            :entries="folder.items"
            :label="`Folder ${folder.name}`"
            empty-label="Drop items here (an empty folder is not shown)"
        />
    </div>
</template>
