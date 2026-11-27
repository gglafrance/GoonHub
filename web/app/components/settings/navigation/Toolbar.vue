<script setup lang="ts">
// Actions on the selected entry, the keyboard and touch alternative to the drag and drop: move
// left/right, move to a zone, into a folder or to Hidden, show again; for a folder, its name,
// its icon and dissolving it (its items take its place).
const editor = useNavLayoutEditorContext();

const entry = computed(() => editor.selected.value);
const folderId = computed(() => (entry.value ? navFolderRefId(entry.value) : null));
const folder = computed(() =>
    folderId.value ? editor.draft.value.folders.find((f) => f.id === folderId.value) : undefined,
);
const item = computed(() =>
    entry.value && !folderId.value ? NAV_ITEMS_BY_ID.get(entry.value) : undefined,
);
const position = computed(() =>
    entry.value ? findNavEntry(editor.draft.value, entry.value) : null,
);
const listLength = computed(() => {
    const loc = position.value?.loc;
    if (!loc) return 0;
    if (loc.kind === 'zone') return editor.draft.value.bars[loc.bar][loc.zone].length;
    if (loc.kind === 'hidden') return editor.draft.value.hidden.length;
    return editor.draft.value.folders.find((f) => f.id === loc.folderId)?.items.length ?? 0;
});
const isHidden = computed(() => position.value?.loc.kind === 'hidden');

const moveOptions = computed(() => {
    const e = entry.value;
    if (!e) return [];
    const options: { value: string; label: string }[] = [];
    for (const bar of NAV_BARS) {
        for (const zone of NAV_ZONES) {
            const loc = { kind: 'zone' as const, bar, zone };
            if (editor.canPlace(e, loc)) {
                options.push({
                    value: navLocationKey(loc),
                    label: `${NAV_BAR_LABELS[bar]}, ${NAV_ZONE_LABELS[zone]}`,
                });
            }
        }
    }
    for (const f of editor.draft.value.folders) {
        const loc = { kind: 'folder' as const, folderId: f.id };
        if (editor.canPlace(e, loc)) {
            options.push({ value: navLocationKey(loc), label: `Folder: ${f.name}` });
        }
    }
    if (editor.canPlace(e, { kind: 'hidden' })) options.push({ value: 'hidden', label: 'Hidden' });
    return options;
});
const currentKey = computed(() => (position.value ? navLocationKey(position.value.loc) : ''));

// Folder name and icon, applied as typed (an invalid name is kept in the field only)
const name = ref('');
watch(
    folder,
    (f) => {
        if (f && f.name !== name.value.trim()) name.value = f.name;
    },
    { immediate: true },
);
const nameValid = computed(() => cleanFolderName(name.value) !== null);

function setDraft(next: typeof editor.draft.value) {
    editor.draft.value = next;
}

function shift(delta: -1 | 1) {
    if (entry.value) setDraft(shiftNavEntry(editor.draft.value, entry.value, delta));
}

function moveTo(key: string) {
    const loc = parseNavLocationKey(key);
    if (entry.value && loc) editor.move(entry.value, loc);
}

function show() {
    if (entry.value) setDraft(showNavItem(editor.draft.value, entry.value));
}

function rename() {
    const f = folder.value;
    if (f && nameValid.value)
        setDraft(updateNavFolder(editor.draft.value, f.id, name.value, f.icon));
}

function setIcon(icon: string) {
    const f = folder.value;
    if (f) setDraft(updateNavFolder(editor.draft.value, f.id, f.name, icon));
}

function dissolve() {
    const f = folder.value;
    if (!f) return;
    setDraft(dissolveNavFolder(editor.draft.value, f.id));
    editor.select(null);
}

const buttonClass = `border-border text-dim hover:border-lava/30 flex h-7 items-center gap-1
    rounded-md border px-2 text-[11px] font-medium transition-all hover:text-white
    disabled:pointer-events-none disabled:opacity-30`;
</script>

<template>
    <div v-if="entry && (item || folder)" class="glass-panel space-y-3 p-4">
        <div class="flex items-center justify-between gap-2">
            <div class="flex min-w-0 items-center gap-2 text-xs font-semibold text-white">
                <Icon
                    :name="folder ? folder.icon || 'heroicons:folder' : item!.icon"
                    size="14"
                    class="text-lava shrink-0"
                />
                <span class="truncate">{{ folder ? `Folder ${folder.name}` : item!.label }}</span>
                <span
                    v-if="item?.required"
                    class="text-dim rounded-full border border-white/10 px-1.5 py-px text-[9px]
                        font-medium tracking-wider uppercase"
                >
                    Always shown
                </span>
            </div>
            <button
                type="button"
                class="text-dim hover:text-white"
                aria-label="Close the selection"
                @click="editor.select(null)"
            >
                <Icon name="heroicons:x-mark" size="14" />
            </button>
        </div>

        <div class="flex flex-wrap items-center gap-2">
            <button
                type="button"
                :class="buttonClass"
                :disabled="!position || position.index === 0"
                aria-label="Move left"
                @click="shift(-1)"
            >
                <Icon name="heroicons:arrow-left" size="12" />
                Left
            </button>
            <button
                type="button"
                :class="buttonClass"
                :disabled="!position || position.index >= listLength - 1"
                aria-label="Move right"
                @click="shift(1)"
            >
                Right
                <Icon name="heroicons:arrow-right" size="12" />
            </button>
            <div class="relative w-56">
                <select
                    :value="currentKey"
                    aria-label="Move to"
                    class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 h-7
                        w-full appearance-none rounded-md border pr-8 pl-2.5 text-[11px] text-white
                        transition-all focus:ring-1 focus:outline-none"
                    @change="moveTo(($event.target as HTMLSelectElement).value)"
                >
                    <option
                        v-for="opt in moveOptions"
                        :key="opt.value"
                        :value="opt.value"
                        class="bg-panel"
                    >
                        {{ opt.label }}
                    </option>
                </select>
                <Icon
                    name="heroicons:chevron-down"
                    size="12"
                    class="text-dim pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2"
                />
            </div>
            <button v-if="isHidden" type="button" :class="buttonClass" @click="show">
                <Icon name="heroicons:eye" size="12" />
                Show again
            </button>
        </div>

        <div v-if="folder" class="border-border space-y-3 border-t pt-3">
            <div class="flex flex-wrap items-end gap-2">
                <label class="flex flex-col gap-1">
                    <span class="text-dim text-[10px]">Name</span>
                    <input
                        v-model="name"
                        type="text"
                        :maxlength="MAX_NAV_FOLDER_NAME_LENGTH"
                        class="border-border bg-void/80 focus:border-lava/40 w-56 rounded-lg border
                            px-3 py-1.5 text-xs text-white focus:outline-none"
                        :class="{ '!border-lava/60': !nameValid }"
                        @input="rename"
                    />
                </label>
                <button type="button" :class="buttonClass" @click="dissolve">
                    <Icon name="heroicons:arrows-pointing-out" size="12" />
                    Dissolve folder
                </button>
            </div>
            <div>
                <div class="text-dim mb-1 text-[10px]">Icon</div>
                <div class="flex flex-wrap gap-1" role="radiogroup" aria-label="Folder icon">
                    <button
                        v-for="icon in NAV_FOLDER_ICONS"
                        :key="icon"
                        type="button"
                        role="radio"
                        :aria-checked="(folder.icon || 'heroicons:folder') === icon"
                        :aria-label="icon.replace('heroicons:', '')"
                        class="flex h-7 w-7 items-center justify-center rounded-md border
                            transition-all"
                        :class="
                            (folder.icon || 'heroicons:folder') === icon
                                ? 'border-lava/50 bg-lava/10 text-lava'
                                : 'border-border text-dim hover:border-lava/30 hover:text-white'
                        "
                        @click="setIcon(icon)"
                    >
                        <Icon :name="icon" size="14" />
                    </button>
                </div>
            </div>
        </div>
        <p v-else-if="item && !item.folderable" class="text-dim text-[10px]">
            This item opens a panel under its button: it stays in a bar (not in a folder).
        </p>
    </div>
</template>
