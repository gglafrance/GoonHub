<script setup lang="ts">
import type { PaletteItem } from '~/types/command-palette';

// Global command palette: Ctrl+K / Cmd+K anywhere, or "/" outside text fields.
const { isOpen, close, remember, clearRecent } = useCommandPalette();
const router = useRouter();

const query = ref('');
const inputRef = ref<HTMLInputElement | null>(null);
const listRef = ref<HTMLElement | null>(null);
const { groups, loading, prepare } = useCommandPaletteResults(query);

// Groups with numbered entries, and the flat list used for keyboard navigation; the key
// keeps an item unique across groups (an actor can be both in "Current scene" and "Actors")
const sections = computed(() => {
    let index = 0;
    return groups.value.map((group, groupIndex) => ({
        group,
        entries: group.items.map((item) => ({
            key: `${group.id}|${item.id}`,
            item,
            groupIndex,
            index: index++,
        })),
    }));
});
const flat = computed(() => sections.value.flatMap((s) => s.entries));
const activeKey = ref<string | null>(null);
const activeIndex = computed(() => {
    const idx = flat.value.findIndex((e) => e.key === activeKey.value);
    return idx === -1 ? 0 : idx;
});

// Until the user moves the highlight, it follows the first entry as results arrive; after
// that it stays on the chosen entry while it is still listed
const userNavigated = ref(false);
watch(flat, (list) => {
    if (!userNavigated.value || !list.some((e) => e.key === activeKey.value)) {
        activeKey.value = list[0]?.key ?? null;
    }
});
watch(query, () => {
    userNavigated.value = false;
    activeKey.value = flat.value[0]?.key ?? null;
});

const onHover = (key: string) => {
    if (activeKey.value === key) return;
    userNavigated.value = true;
    activeKey.value = key;
};

const groupStart = (groupIndex: number) => flat.value.findIndex((e) => e.groupIndex === groupIndex);

const setActive = (index: number) => {
    const entry = flat.value[index];
    if (!entry) return;
    userNavigated.value = true;
    activeKey.value = entry.key;
    nextTick(() => {
        listRef.value
            ?.querySelector(`[data-index="${index}"]`)
            ?.scrollIntoView({ block: 'nearest' });
    });
};

const execute = async (item: PaletteItem, newTab = false) => {
    remember(item);
    close();
    if (item.to) {
        if (newTab) window.open(router.resolve(item.to).href, '_blank', 'noopener');
        else await router.push(item.to);
    } else if (item.run) {
        try {
            await item.run();
        } catch (err: unknown) {
            console.error('Command palette action failed:', err);
        }
    }
};

const onInputKeydown = (e: KeyboardEvent) => {
    if (e.isComposing) return;
    const count = flat.value.length;
    if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        close();
    } else if (e.key === 'ArrowDown' && count) {
        e.preventDefault();
        setActive((activeIndex.value + 1) % count);
    } else if (e.key === 'ArrowUp' && count) {
        e.preventDefault();
        setActive((activeIndex.value - 1 + count) % count);
    } else if (e.key === 'Tab' && count) {
        // Jump to the first entry of the next (Shift: previous) group
        e.preventDefault();
        const current = flat.value[activeIndex.value]?.groupIndex ?? 0;
        const total = groups.value.length;
        const next = (current + (e.shiftKey ? total - 1 : 1)) % total;
        setActive(groupStart(next));
    } else if (e.key === 'Enter') {
        e.preventDefault();
        const entry = flat.value[activeIndex.value];
        if (entry) execute(entry.item, e.ctrlKey || e.metaKey);
    }
};

const onSelect = (index: number, event: MouseEvent) => {
    const entry = flat.value[index];
    if (entry) execute(entry.item, event.ctrlKey || event.metaKey);
};

watch(isOpen, (value) => {
    if (!value) return;
    query.value = '';
    userNavigated.value = false;
    activeKey.value = flat.value[0]?.key ?? null;
    prepare();
    nextTick(() => inputRef.value?.focus());
});

useCommandPaletteHotkey();
</script>

<template>
    <Teleport to="body">
        <Transition
            enter-active-class="transition duration-150 ease-out"
            enter-from-class="opacity-0"
            enter-to-class="opacity-100"
            leave-active-class="transition duration-100 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
        >
            <div
                v-if="isOpen"
                class="fixed inset-0 z-100 flex items-start justify-center px-4 pt-[12vh]"
            >
                <div class="bg-void/70 absolute inset-0 backdrop-blur-sm" @click="close()" />

                <div
                    class="border-border-hover bg-panel/90 relative flex max-h-[70vh] w-full
                        max-w-2xl flex-col overflow-hidden rounded-xl border
                        shadow-[0_0_0_1px_rgba(255,77,77,0.06),0_24px_80px_-12px_rgba(0,0,0,0.9)]
                        backdrop-blur-xl"
                    role="dialog"
                    aria-modal="true"
                    aria-label="Command palette"
                >
                    <!-- Search input -->
                    <div class="border-border flex items-center gap-3 border-b px-4">
                        <Icon
                            name="heroicons:magnifying-glass"
                            size="18"
                            class="text-lava shrink-0"
                        />
                        <input
                            ref="inputRef"
                            v-model="query"
                            type="text"
                            role="combobox"
                            aria-expanded="true"
                            aria-controls="command-palette-list"
                            placeholder="Search scenes, actors, pages, actions..."
                            autocomplete="off"
                            spellcheck="false"
                            class="h-13 flex-1 bg-transparent text-sm text-white outline-none
                                placeholder:text-white/30"
                            @keydown="onInputKeydown"
                        />
                        <Icon
                            v-if="loading"
                            name="svg-spinners:ring-resize"
                            size="14"
                            class="text-lava shrink-0"
                        />
                        <kbd
                            class="border-border bg-surface text-dim shrink-0 rounded border px-1.5
                                py-0.5 font-mono text-[10px]"
                        >
                            Esc
                        </kbd>
                    </div>

                    <!-- Results -->
                    <div
                        id="command-palette-list"
                        ref="listRef"
                        role="listbox"
                        class="flex-1 overflow-y-auto p-2"
                        @mousedown.prevent
                    >
                        <template
                            v-for="({ group, entries }, groupIndex) in sections"
                            :key="group.id"
                        >
                            <div
                                class="flex items-center justify-between px-2.5 pt-2.5 pb-1"
                                :class="groupIndex > 0 ? 'border-border mt-1 border-t' : ''"
                            >
                                <span
                                    class="text-dim text-[10px] font-semibold tracking-wider
                                        uppercase"
                                >
                                    {{ group.label }}
                                </span>
                                <button
                                    v-if="group.id === 'recent'"
                                    type="button"
                                    class="text-dim hover:text-lava text-[10px] transition-colors"
                                    @mousedown.prevent
                                    @click="clearRecent()"
                                >
                                    Clear
                                </button>
                            </div>
                            <CommandPaletteItem
                                v-for="entry in entries"
                                :key="entry.key"
                                :data-index="entry.index"
                                :item="entry.item"
                                :active="entry.index === activeIndex"
                                @hover="onHover(entry.key)"
                                @select="onSelect(entry.index, $event)"
                            />
                        </template>

                        <div
                            v-if="groups.length === 0"
                            class="text-dim px-4 py-10 text-center text-xs"
                        >
                            <template v-if="query.trim()">No results</template>
                            <template v-else>Type to search your library</template>
                        </div>
                    </div>

                    <CommandPaletteFooter />
                </div>
            </div>
        </Transition>
    </Teleport>
</template>
