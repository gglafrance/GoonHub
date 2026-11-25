<script setup lang="ts">
import type { HomepageConfig, HomepageOpenOn } from '~/types/homepage';

// Manages the homepage tabs: add (blank or template), rename, icon, reorder, delete, default.
const props = defineProps<{
    config: HomepageConfig;
}>();

const selected = defineModel<string>('selected', { required: true });

const showTemplates = ref(false);
const iconPickerFor = ref<string | null>(null);
const confirmDeleteId = ref<string | null>(null);

const tabs = computed(() => props.config.tabs);

const addTab = (templateId: string) => {
    const template = HOMEPAGE_TAB_TEMPLATES.find((t) => t.id === templateId);
    if (!template || tabs.value.length >= 12) return;
    const tab = createTabFromTemplate(template);

    props.config.tabs.push(tab);
    selected.value = tab.id;
    showTemplates.value = false;
};

const move = (index: number, offset: number) => {
    const target = index + offset;
    if (target < 0 || target >= tabs.value.length) return;
    const next = [...tabs.value];
    [next[index], next[target]] = [next[target]!, next[index]!];

    props.config.tabs = next;
};

const remove = (id: string) => {
    if (tabs.value.length <= 1) return;

    props.config.tabs = tabs.value.filter((t) => t.id !== id);
    if (props.config.default_tab_id === id) {
        props.config.default_tab_id = props.config.tabs[0]!.id;
    }
    if (selected.value === id) selected.value = props.config.tabs[0]!.id;
    confirmDeleteId.value = null;
};

const requestRemove = (id: string) => {
    const tab = tabs.value.find((t) => t.id === id);
    if (tab && tab.sections.length > 0 && confirmDeleteId.value !== id) {
        confirmDeleteId.value = id;
        return;
    }
    remove(id);
};

const setDefault = (id: string) => {
    props.config.default_tab_id = id;
};

const setIcon = (id: string, icon: string) => {
    const tab = tabs.value.find((t) => t.id === id);
    if (tab) tab.icon = icon;
    iconPickerFor.value = null;
};

const openOn = computed({
    get: () => props.config.open_on ?? 'last',
    set: (v: HomepageOpenOn) => {
        props.config.open_on = v;
    },
});
</script>

<template>
    <div class="glass-panel overflow-hidden">
        <div class="border-border flex items-center justify-between border-b px-5 py-4">
            <div class="flex items-center gap-3">
                <div class="bg-lava/10 flex h-8 w-8 items-center justify-center rounded-lg">
                    <Icon name="heroicons:rectangle-group" size="16" class="text-lava" />
                </div>
                <div>
                    <h3 class="text-sm font-semibold text-white">Tabs</h3>
                    <p class="text-dim text-[11px]">
                        {{ tabs.length }} tab{{ tabs.length > 1 ? 's' : '' }} · select one to edit
                        its sections
                    </p>
                </div>
            </div>
            <div class="relative">
                <button
                    class="border-border hover:border-lava/30 flex items-center gap-1.5 rounded-lg
                        border px-3 py-2 text-xs font-medium text-white transition-all
                        disabled:opacity-40"
                    :disabled="tabs.length >= 12"
                    @click="showTemplates = !showTemplates"
                >
                    <Icon name="heroicons:plus" size="14" class="text-lava" />
                    Add Tab
                </button>
                <div
                    v-if="showTemplates"
                    class="border-border bg-panel absolute top-full right-0 z-20 mt-1 w-72
                        rounded-lg border p-1.5 shadow-xl"
                >
                    <button
                        v-for="template in HOMEPAGE_TAB_TEMPLATES"
                        :key="template.id"
                        class="flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-left
                            transition-colors hover:bg-white/5"
                        @click="addTab(template.id)"
                    >
                        <Icon :name="template.icon" size="14" class="text-lava mt-0.5 shrink-0" />
                        <div>
                            <div class="text-xs text-white">{{ template.name }}</div>
                            <div class="text-dim text-[10px]">{{ template.description }}</div>
                        </div>
                    </button>
                </div>
            </div>
        </div>

        <div class="space-y-1.5 p-4">
            <div
                v-for="(tab, index) in tabs"
                :key="tab.id"
                class="flex items-center gap-2 rounded-lg border px-2.5 py-1.5 transition-all"
                :class="
                    tab.id === selected
                        ? 'border-lava/40 bg-lava/5'
                        : 'border-border cursor-pointer hover:border-white/20'
                "
                @click="selected = tab.id"
            >
                <div class="relative">
                    <button
                        class="text-dim flex h-7 w-7 items-center justify-center rounded-md
                            transition-colors hover:bg-white/5 hover:text-white"
                        title="Change icon"
                        @click.stop="iconPickerFor = iconPickerFor === tab.id ? null : tab.id"
                    >
                        <Icon :name="tab.icon || 'heroicons:squares-2x2'" size="15" />
                    </button>
                    <div
                        v-if="iconPickerFor === tab.id"
                        class="border-border bg-panel absolute top-full left-0 z-20 mt-1 grid w-44
                            grid-cols-6 gap-1 rounded-lg border p-1.5 shadow-xl"
                    >
                        <button
                            v-for="icon in HOMEPAGE_TAB_ICONS"
                            :key="icon"
                            class="text-dim flex h-6 w-6 items-center justify-center rounded
                                hover:bg-white/10 hover:text-white"
                            @click.stop="setIcon(tab.id, icon)"
                        >
                            <Icon :name="icon" size="13" />
                        </button>
                    </div>
                </div>
                <input
                    v-model="tab.name"
                    maxlength="40"
                    class="min-w-0 flex-1 bg-transparent text-xs text-white focus:outline-none"
                    @click.stop
                />
                <span class="text-dim font-mono text-[10px]">
                    {{ tab.sections.length }} section{{ tab.sections.length === 1 ? '' : 's' }}
                </span>
                <button
                    class="p-1 transition-colors"
                    :class="
                        config.default_tab_id === tab.id
                            ? 'text-amber-400'
                            : 'text-dim hover:text-amber-400'
                    "
                    :title="config.default_tab_id === tab.id ? 'Default tab' : 'Make default tab'"
                    @click.stop="setDefault(tab.id)"
                >
                    <Icon
                        :name="
                            config.default_tab_id === tab.id
                                ? 'heroicons:star-solid'
                                : 'heroicons:star'
                        "
                        size="13"
                    />
                </button>
                <button
                    class="text-dim p-1 hover:text-white disabled:opacity-20"
                    :disabled="index === 0"
                    title="Move left"
                    @click.stop="move(index, -1)"
                >
                    <Icon name="heroicons:chevron-up" size="13" />
                </button>
                <button
                    class="text-dim p-1 hover:text-white disabled:opacity-20"
                    :disabled="index === tabs.length - 1"
                    title="Move right"
                    @click.stop="move(index, 1)"
                >
                    <Icon name="heroicons:chevron-down" size="13" />
                </button>
                <button
                    class="p-1 transition-colors disabled:opacity-20"
                    :class="confirmDeleteId === tab.id ? 'text-lava' : 'text-dim hover:text-lava'"
                    :disabled="tabs.length <= 1"
                    :title="
                        confirmDeleteId === tab.id
                            ? 'Click again to delete the tab and its sections'
                            : 'Delete tab'
                    "
                    @click.stop="requestRemove(tab.id)"
                >
                    <Icon name="heroicons:trash" size="13" />
                </button>
            </div>

            <div class="border-border mt-3 flex items-center justify-between border-t pt-3">
                <div>
                    <div class="text-xs text-white">Open on</div>
                    <div class="text-dim text-[11px]">Tab shown when you open the homepage</div>
                </div>
                <div class="flex gap-1">
                    <button
                        v-for="option in [
                            { value: 'last', label: 'Last visited' },
                            { value: 'default', label: 'Default tab' },
                        ] as const"
                        :key="option.value"
                        class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                        :class="
                            openOn === option.value
                                ? 'border-lava/40 bg-lava/10 text-lava'
                                : 'border-border text-dim hover:text-white'
                        "
                        @click="openOn = option.value"
                    >
                        {{ option.label }}
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
