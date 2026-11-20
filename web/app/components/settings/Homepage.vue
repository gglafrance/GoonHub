<script setup lang="ts">
import type { HomepageSection, HomepageTab } from '~/types/homepage';

const settingsStore = useSettingsStore();

const showModal = ref(false);
const editingSection = ref<HomepageSection | null>(null);

const config = computed(() => settingsStore.draft?.homepage_config ?? null);

// Tab whose sections are edited
const selectedTabId = ref('');
const currentTab = computed<HomepageTab | null>(() => {
    const tabs = config.value?.tabs ?? [];
    return tabs.find((t) => t.id === selectedTabId.value) ?? tabs[0] ?? null;
});
const sections = computed<HomepageSection[]>(() => currentTab.value?.sections ?? []);

watch(
    () => config.value?.tabs,
    (tabs) => {
        if (tabs?.length && !tabs.some((t) => t.id === selectedTabId.value)) {
            selectedTabId.value = config.value?.default_tab_id || tabs[0]!.id;
        }
    },
    { immediate: true },
);

function setSections(next: HomepageSection[]) {
    if (!currentTab.value) return;
    next.forEach((s, i) => (s.order = i));
    currentTab.value.sections = next;
}

function handleAddSection() {
    editingSection.value = null;
    showModal.value = true;
}

function handleEditSection(section: HomepageSection) {
    editingSection.value = { ...section };
    showModal.value = true;
}

function handleDeleteSection(sectionId: string) {
    setSections(sections.value.filter((s) => s.id !== sectionId));
}

function handleToggleSection(sectionId: string) {
    const section = sections.value.find((s) => s.id === sectionId);
    if (section) {
        section.enabled = !section.enabled;
    }
}

function handleMoveSection(sectionId: string, direction: 'up' | 'down') {
    const index = sections.value.findIndex((s) => s.id === sectionId);
    if (index === -1) return;

    const newIndex = direction === 'up' ? index - 1 : index + 1;
    if (newIndex < 0 || newIndex >= sections.value.length) return;

    const next = [...sections.value];
    const temp = next[index]!;
    next[index] = next[newIndex]!;
    next[newIndex] = temp;
    setSections(next);
}

// Moves a section to the end of another tab
function handleMoveToTab(sectionId: string, tabId: string) {
    const target = config.value?.tabs.find((t) => t.id === tabId);
    const section = sections.value.find((s) => s.id === sectionId);
    if (!target || !section || target.id === currentTab.value?.id) return;
    setSections(sections.value.filter((s) => s.id !== sectionId));
    section.order = target.sections.length;
    target.sections.push(section);
}

function handleSectionSaved(section: HomepageSection) {
    const existingIndex = sections.value.findIndex((s) => s.id === section.id);
    if (existingIndex >= 0) {
        const next = [...sections.value];
        next[existingIndex] = section;
        setSections(next);
    } else {
        setSections([...sections.value, section]);
    }
    showModal.value = false;
}

const otherTabs = computed(() =>
    (config.value?.tabs ?? []).filter((t) => t.id !== currentTab.value?.id),
);
const enabledCount = computed(() => sections.value.filter((s) => s.enabled).length);
const totalCount = computed(() => sections.value.length);
</script>

<template>
    <div class="space-y-6">
        <template v-if="config">
            <!-- General Settings -->
            <div class="glass-panel overflow-hidden">
                <div class="border-border flex items-center gap-3 border-b px-5 py-4">
                    <div class="bg-lava/10 flex h-8 w-8 items-center justify-center rounded-lg">
                        <Icon name="heroicons:cog-6-tooth" size="16" class="text-lava" />
                    </div>
                    <div>
                        <h3 class="text-sm font-semibold text-white">General Settings</h3>
                        <p class="text-dim text-[11px]">Configure homepage behavior</p>
                    </div>
                </div>
                <div class="p-5">
                    <label class="flex cursor-pointer items-center justify-between">
                        <div class="flex items-center gap-3">
                            <Icon name="heroicons:cloud-arrow-up" size="18" class="text-dim" />
                            <div>
                                <span class="block text-xs font-medium text-white"
                                    >Upload Section</span
                                >
                                <span class="text-dim text-[11px]"
                                    >Show quick upload area on homepage</span
                                >
                            </div>
                        </div>
                        <UiToggle v-model="config.show_upload" />
                    </label>
                </div>
            </div>

            <!-- Tabs -->
            <SettingsHomepageTabsEditor v-model:selected="selectedTabId" :config="config" />

            <!-- Sections -->
            <div class="glass-panel overflow-hidden">
                <div class="border-border flex items-center justify-between border-b px-5 py-4">
                    <div class="flex items-center gap-3">
                        <div class="bg-lava/10 flex h-8 w-8 items-center justify-center rounded-lg">
                            <Icon name="heroicons:squares-2x2" size="16" class="text-lava" />
                        </div>
                        <div>
                            <h3 class="text-sm font-semibold text-white">
                                Sections
                                <span v-if="currentTab" class="text-lava">
                                    · {{ currentTab.name }}
                                </span>
                            </h3>
                            <p class="text-dim text-[11px]">
                                <span v-if="totalCount > 0">
                                    {{ enabledCount }} of {{ totalCount }} sections enabled
                                </span>
                                <span v-else>Organize your homepage content</span>
                            </p>
                        </div>
                    </div>
                    <button
                        class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-3
                            py-2 text-xs font-medium text-white transition-all hover:scale-[1.02]
                            active:scale-[0.98]"
                        @click="handleAddSection"
                    >
                        <Icon name="heroicons:plus" size="14" />
                        Add Section
                    </button>
                </div>

                <!-- Empty State -->
                <div
                    v-if="sections.length === 0"
                    class="flex flex-col items-center justify-center py-12"
                >
                    <div
                        class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl
                            bg-white/5"
                    >
                        <Icon name="heroicons:rectangle-stack" size="28" class="text-dim" />
                    </div>
                    <h4 class="mb-1 text-sm font-medium text-white">No sections yet</h4>
                    <p class="text-dim mb-4 max-w-xs text-center text-xs">
                        Add sections to customize what content appears on your homepage
                    </p>
                    <button
                        class="border-border hover:border-lava/30 hover:bg-lava/5 flex items-center
                            gap-2 rounded-lg border px-4 py-2 text-xs font-medium text-white
                            transition-all"
                        @click="handleAddSection"
                    >
                        <Icon name="heroicons:plus" size="14" class="text-lava" />
                        Create your first section
                    </button>
                </div>

                <!-- Section List -->
                <div v-else class="divide-border divide-y">
                    <TransitionGroup name="list" tag="div" class="divide-border divide-y">
                        <div
                            v-for="(section, index) in sections"
                            :key="section.id"
                            class="flex items-center"
                        >
                            <SettingsHomepageSectionItem
                                class="min-w-0 flex-1"
                                :section="section"
                                :is-first="index === 0"
                                :is-last="index === sections.length - 1"
                                @edit="handleEditSection"
                                @delete="handleDeleteSection"
                                @toggle="handleToggleSection"
                                @move="handleMoveSection"
                            />
                            <select
                                v-if="otherTabs.length"
                                class="border-border bg-panel text-dim mr-4 w-28 shrink-0 rounded-md
                                    border px-1.5 py-1 text-[10px]"
                                title="Move to another tab"
                                :value="''"
                                @change="
                                    handleMoveToTab(
                                        section.id,
                                        ($event.target as HTMLSelectElement).value,
                                    )
                                "
                            >
                                <option value="" disabled>Move to...</option>
                                <option v-for="tab in otherTabs" :key="tab.id" :value="tab.id">
                                    {{ tab.name }}
                                </option>
                            </select>
                        </div>
                    </TransitionGroup>
                </div>
            </div>
        </template>

        <!-- Section Modal -->
        <SettingsHomepageSectionModal
            v-if="showModal"
            :section="editingSection"
            @close="showModal = false"
            @save="handleSectionSaved"
        />
    </div>
</template>

<style scoped>
.list-move,
.list-enter-active,
.list-leave-active {
    transition: all 0.3s ease;
}

.list-enter-from,
.list-leave-to {
    opacity: 0;
    transform: translateX(-20px);
}

.list-leave-active {
    position: absolute;
}
</style>
