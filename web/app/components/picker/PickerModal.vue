<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type { MarkerWithScene } from '~/types/marker';
import type { PickedItem, PickerAction, PickerExclude, PickerTab } from '~/utils/picker';
import type PickerScenes from '~/components/picker/Scenes.vue';
import type PickerMarkers from '~/components/picker/Markers.vue';

/**
 * Search picker: a large modal with the search page interface to pick videos and markers.
 *
 *   <PickerModal
 *       :visible="open"
 *       :tabs="['scenes', 'markers']"     tabs shown, the first one opens (default: both)
 *       multiple                           several items (else one: a click replaces it)
 *       :max="3"                           most items (multiple), 0 = no limit
 *       :exclude="{ scenes: [1], markers: [4] }"  already picked: marked with `excluded-label`
 *       excluded-selectable                excluded items may still be picked (marked only)
 *       title="Add to the queue"
 *       confirm-label="Add"                primary button ("Add 3")
 *       :actions="[{ id: 'play', label: 'Play now', icon: 'heroicons:play' }]"
 *       hint="..."                         small text next to the buttons
 *       @picked="(items, action) => ..."   items: PickedItem[] in selection order; action:
 *                                          'confirm' or the id of an extra action
 *       @close="open = false"
 *   />
 *
 * PickedItem is `{ kind: 'scene', scene }` or `{ kind: 'marker', marker }` (utils/picker.ts).
 * The modal does not close itself on pick: the parent closes it (or keeps it open).
 * The Videos tab runs on its own search store instance (the search page state and the URL are
 * untouched); RBAC follows the search page (no size filter or sort without files:view_size).
 * Keys: Esc closes, Enter in a search field searches, Ctrl+Enter confirms.
 */
const props = withDefaults(
    defineProps<{
        visible: boolean;
        tabs?: PickerTab[];
        multiple?: boolean;
        max?: number;
        exclude?: PickerExclude;
        excludedSelectable?: boolean;
        excludedLabel?: string;
        title?: string;
        confirmLabel?: string;
        actions?: PickerAction[];
        hint?: string;
    }>(),
    {
        tabs: () => ['scenes', 'markers'],
        max: 0,
        exclude: undefined,
        excludedLabel: 'Added',
        title: 'Pick',
        confirmLabel: 'Add',
        actions: () => [],
        hint: undefined,
    },
);

const emit = defineEmits<{
    picked: [items: PickedItem[], action: string];
    close: [];
}>();

const TAB_LABELS: Record<PickerTab, { label: string; icon: string }> = {
    scenes: { label: 'Videos', icon: 'heroicons:film' },
    markers: { label: 'Markers', icon: 'heroicons:bookmark' },
};

const tab = ref<PickerTab>('scenes');
const selection = ref<PickedItem[]>([]);
const root = ref<HTMLElement | null>(null);
const scenesTab = ref<InstanceType<typeof PickerScenes> | null>(null);
const markersTab = ref<InstanceType<typeof PickerMarkers> | null>(null);

const excluded = computed(() => excludedKeys(props.exclude));
const options = computed(() => ({
    multiple: !!props.multiple,
    max: props.max,
    excluded: excluded.value,
    excludedSelectable: props.excludedSelectable,
}));
const selectedKeys = computed(() => new Set(selection.value.map(pickerKey)));

const toggle = (item: PickedItem) => {
    selection.value = togglePick(selection.value, item, options.value);
};
const remove = (item: PickedItem) => {
    selection.value = selection.value.filter((i) => pickerKey(i) !== pickerKey(item));
};

// Card states, per kind
const sceneItem = (scene: SceneListItem): PickedItem => ({ kind: 'scene', scene });
const markerItem = (marker: MarkerWithScene): PickedItem => ({ kind: 'marker', marker });
const isSelected = (item: PickedItem) => selectedKeys.value.has(pickerKey(item));
const markedLabel = (item: PickedItem) =>
    excluded.value.has(pickerKey(item)) ? props.excludedLabel : undefined;
const isDisabled = (item: PickedItem) =>
    !isSelected(item) && !canPick(selection.value, item, options.value);

const confirm = (action = 'confirm') => {
    if (selection.value.length === 0) return;
    emit('picked', [...selection.value], action);
};

// The search field gets the focus with a mouse; on touch screens it would open the keyboard
const focusSearch = () =>
    nextTick(() => {
        if (!window.matchMedia('(pointer: fine)').matches) return;
        (tab.value === 'scenes' ? scenesTab.value : markersTab.value)?.focus();
    });

const onKeydown = (e: KeyboardEvent) => {
    // The page below (player shortcuts, command palette...) gets no key while the picker is open
    e.stopPropagation();
    if (e.key === 'Escape') {
        e.preventDefault();
        emit('close');
    } else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        confirm();
    }
};

// The page below does not scroll while the picker is open
const lockScroll = (lock: boolean) => {
    if (import.meta.client) document.body.style.overflow = lock ? 'hidden' : '';
};

watch(
    () => props.visible,
    (open, wasOpen) => {
        if (open || wasOpen) lockScroll(open);
        if (!open) return;
        selection.value = [];
        tab.value = props.tabs[0] ?? 'scenes';
        nextTick(() => {
            root.value?.focus();
            focusSearch();
        });
    },
    { immediate: true },
);
watch(tab, focusSearch);
onBeforeUnmount(() => {
    if (props.visible) lockScroll(false);
});
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex bg-black/70 backdrop-blur-sm sm:p-4 lg:p-8"
            @click.self="emit('close')"
        >
            <div
                ref="root"
                tabindex="-1"
                role="dialog"
                aria-modal="true"
                :aria-label="title"
                class="border-border bg-void/95 mx-auto flex h-full w-full max-w-[1800px] flex-col
                    overflow-hidden shadow-2xl outline-none sm:rounded-xl sm:border"
                @keydown="onKeydown"
            >
                <!-- Header: title, tabs, close -->
                <div class="border-border flex items-center gap-3 border-b px-4 py-2.5 sm:px-5">
                    <h2 class="truncate text-sm font-semibold text-white">{{ title }}</h2>
                    <div
                        v-if="tabs.length > 1"
                        class="border-border bg-panel flex shrink-0 items-center rounded-lg border
                            p-0.5"
                        role="tablist"
                    >
                        <button
                            v-for="t in tabs"
                            :key="t"
                            role="tab"
                            :aria-selected="tab === t"
                            class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px]
                                font-medium transition-all"
                            :class="
                                tab === t ? 'bg-lava/15 text-lava' : 'text-dim hover:text-white'
                            "
                            @click="tab = t"
                        >
                            <Icon :name="TAB_LABELS[t].icon" size="13" />
                            {{ TAB_LABELS[t].label }}
                        </button>
                    </div>
                    <button
                        class="text-dim ml-auto flex h-8 w-8 items-center justify-center rounded-lg
                            transition-colors hover:bg-white/5 hover:text-white"
                        title="Close (Esc)"
                        @click="emit('close')"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Body: the tab -->
                <div class="min-h-0 flex-1">
                    <PickerScenes
                        v-if="tab === 'scenes'"
                        ref="scenesTab"
                        :is-selected="(s) => isSelected(sceneItem(s))"
                        :marked-label="(s) => markedLabel(sceneItem(s))"
                        :is-disabled="(s) => isDisabled(sceneItem(s))"
                        @toggle="(s) => toggle(sceneItem(s))"
                    />
                    <PickerMarkers
                        v-else
                        ref="markersTab"
                        :is-selected="(m) => isSelected(markerItem(m))"
                        :marked-label="(m) => markedLabel(markerItem(m))"
                        :is-disabled="(m) => isDisabled(markerItem(m))"
                        @toggle="(m) => toggle(markerItem(m))"
                    />
                </div>

                <!-- Footer: selection and actions -->
                <div
                    class="border-border bg-panel/60 flex flex-col gap-2 border-t px-4 py-2.5
                        backdrop-blur-md sm:flex-row sm:items-center sm:px-5"
                >
                    <PickerTray
                        :items="selection"
                        :max="multiple ? max : 0"
                        @remove="remove"
                        @clear="selection = []"
                    />
                    <div class="flex shrink-0 items-center justify-end gap-2">
                        <span v-if="hint" class="text-dim hidden text-[11px] md:inline">
                            {{ hint }}
                        </span>
                        <button
                            class="border-border text-dim rounded-lg border px-3 py-1.5 text-xs
                                transition-colors hover:text-white"
                            @click="emit('close')"
                        >
                            Cancel
                        </button>
                        <button
                            v-for="a in actions"
                            :key="a.id"
                            class="border-lava/30 bg-lava/10 hover:bg-lava/20 flex items-center
                                gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium text-white
                                transition-colors disabled:opacity-40"
                            :disabled="selection.length === 0"
                            @click="confirm(a.id)"
                        >
                            <Icon v-if="a.icon" :name="a.icon" size="13" />
                            {{ a.label }}
                        </button>
                        <button
                            class="bg-lava hover:bg-lava/90 flex items-center gap-1.5 rounded-lg
                                px-3 py-1.5 text-xs font-medium text-white transition-colors
                                disabled:opacity-40"
                            :disabled="selection.length === 0"
                            title="Ctrl+Enter"
                            @click="confirm()"
                        >
                            <Icon name="heroicons:check" size="13" />
                            {{ confirmText(confirmLabel, selection.length, !!multiple) }}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    </Teleport>
</template>
