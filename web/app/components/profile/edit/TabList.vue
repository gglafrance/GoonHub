<script setup lang="ts">
import type { ProfileTabConfig } from '~/types/profile';

// The tabs of the profile in the editor: order (drag and drop or arrows), shown or hidden, the
// title and icon of the custom tabs, add and delete custom tabs. A click selects the tab whose
// sections are edited below. The profile tab is always shown.
const props = defineProps<{ tabs: ProfileTabConfig[]; maxTabs: number }>();
const emit = defineEmits<{ update: [tabs: ProfileTabConfig[]] }>();
const selected = defineModel<string>('selected', { required: true });

const ICONS = [
    'heroicons:squares-2x2',
    'heroicons:fire',
    'heroicons:sparkles',
    'heroicons:star',
    'heroicons:heart',
    'heroicons:film',
    'heroicons:trophy',
    'heroicons:bolt',
    'heroicons:eye',
    'heroicons:tag',
    'heroicons:user-group',
    'fluent-emoji-high-contrast:sweat-droplets',
];
const iconOptions = ICONS.map((i) => ({ value: i, label: i.split(':')[1] ?? i }));

const BUILTIN_TITLES: Record<string, string> = {
    profile: 'Profile',
    likes: 'Likes',
    ratings: 'Ratings',
    jizz: 'Jizz',
};

const titleOf = (t: ProfileTabConfig) => (t.kind === 'custom' ? t.title : BUILTIN_TITLES[t.kind]);

function patch(i: number, change: Partial<ProfileTabConfig>) {
    const next = props.tabs.slice();
    next[i] = { ...next[i]!, ...change };
    emit('update', next);
}

function move(from: number, to: number) {
    emit('update', moveItem(props.tabs, from, to));
}
const drag = useDragReorder(move);

function addTab() {
    const id = newLayoutId('t', layoutIds(props.tabs));
    const tab: ProfileTabConfig = {
        id,
        kind: 'custom',
        title: 'New tab',
        icon: 'heroicons:squares-2x2',
        visible: true,
        sections: [newSection(newLayoutId('s', [id, ...layoutIds(props.tabs)]))],
    };
    emit('update', [...props.tabs, tab]);
    selected.value = id;
}

const confirmDelete = ref<ProfileTabConfig | null>(null);
function remove() {
    const tab = confirmDelete.value;
    confirmDelete.value = null;
    if (!tab) return;
    emit(
        'update',
        props.tabs.filter((t) => t.id !== tab.id),
    );
    if (selected.value === tab.id) selected.value = 'profile';
}
</script>

<template>
    <div class="space-y-2">
        <div class="text-dim text-[10px] tracking-wider uppercase">Tabs</div>
        <div class="border-border divide-border divide-y rounded-lg border">
            <div
                v-for="(tab, i) in props.tabs"
                :key="tab.id"
                v-bind="drag.handlers(i)"
                class="flex flex-wrap items-center gap-2 px-3 py-2 transition-colors"
                :class="[
                    selected === tab.id ? 'bg-lava/5' : 'hover:bg-white/[0.02]',
                    drag.overIndex.value === i ? 'bg-lava/10' : '',
                    tab.visible ? '' : 'opacity-60',
                ]"
            >
                <Icon
                    name="heroicons:bars-3"
                    size="14"
                    class="text-dim hidden cursor-grab sm:block"
                />
                <button
                    type="button"
                    class="flex min-w-0 flex-1 items-center gap-2 text-left"
                    :aria-pressed="selected === tab.id"
                    @click="selected = tab.id"
                >
                    <Icon :name="tab.icon || 'heroicons:squares-2x2'" size="14" class="text-lava" />
                    <span class="truncate text-xs font-medium text-white">{{ titleOf(tab) }}</span>
                    <span v-if="tab.kind !== 'custom'" class="text-dim text-[10px]">built-in</span>
                    <span v-if="selected === tab.id" class="text-lava text-[10px]">editing</span>
                </button>
                <template v-if="tab.kind === 'custom'">
                    <input
                        :value="tab.title"
                        maxlength="40"
                        aria-label="Tab title"
                        class="border-border w-36 rounded-md border bg-white/[0.03] px-2 py-1
                            text-[11px] text-white"
                        @input="
                            patch(i, { title: ($event.target as HTMLInputElement).value, slug: '' })
                        "
                    />
                    <div class="w-36">
                        <UiSelectMenu
                            :model-value="tab.icon || ICONS[0]!"
                            :options="iconOptions"
                            @update:model-value="patch(i, { icon: String($event) })"
                        />
                    </div>
                </template>
                <UiToggle
                    v-if="tab.kind !== 'profile'"
                    :model-value="tab.visible"
                    title="Shown on the profile"
                    @update:model-value="patch(i, { visible: $event })"
                />
                <div class="flex flex-col">
                    <button
                        class="text-dim hover:text-white disabled:opacity-30"
                        :disabled="i === 0"
                        title="Move left"
                        @click="move(i, i - 1)"
                    >
                        <Icon name="heroicons:chevron-up" size="12" />
                    </button>
                    <button
                        class="text-dim hover:text-white disabled:opacity-30"
                        :disabled="i === props.tabs.length - 1"
                        title="Move right"
                        @click="move(i, i + 1)"
                    >
                        <Icon name="heroicons:chevron-down" size="12" />
                    </button>
                </div>
                <button
                    v-if="tab.kind === 'custom'"
                    class="text-dim hover:text-lava"
                    title="Delete the tab"
                    @click="confirmDelete = tab"
                >
                    <Icon name="heroicons:trash" size="13" />
                </button>
            </div>
        </div>
        <button
            class="border-border text-dim hover:border-lava/40 hover:text-lava flex items-center
                gap-1 rounded-md border border-dashed px-2.5 py-1 text-[11px] transition-colors
                disabled:opacity-40"
            :disabled="props.tabs.length >= props.maxTabs"
            @click="addTab"
        >
            <Icon name="heroicons:plus" size="12" />
            Add a tab
            <span class="font-mono">({{ props.tabs.length }}/{{ props.maxTabs }})</span>
        </button>
        <UiConfirmModal
            :visible="!!confirmDelete"
            title="Delete this tab?"
            confirm-label="Delete"
            @close="confirmDelete = null"
            @confirm="remove"
        >
            <p class="text-dim text-xs">
                “{{ confirmDelete?.title }}” and its blocks leave the profile when you save. Your
                custom lists are kept.
            </p>
        </UiConfirmModal>
    </div>
</template>
