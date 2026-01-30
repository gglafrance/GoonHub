<script setup lang="ts">
import type { HomepageConfig, HomepageSection } from '~/types/homepage';

const { getHomepageConfig, updateHomepageConfig } = useApiSettings();
const { message, error, clearMessages } = useSettingsMessage();

const config = ref<HomepageConfig | null>(null);
const loading = ref(false);
const saving = ref(false);
const showModal = ref(false);
const editingSection = ref<HomepageSection | null>(null);
const hasChanges = ref(false);

// Track original config for change detection
const originalConfig = ref<string>('');

onMounted(async () => {
    await loadConfig();
});

async function loadConfig() {
    loading.value = true;
    try {
        config.value = await getHomepageConfig();
        originalConfig.value = JSON.stringify(config.value);
        hasChanges.value = false;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load homepage config';
    } finally {
        loading.value = false;
    }
}

// Watch for changes
watch(
    config,
    (newConfig) => {
        if (newConfig && originalConfig.value) {
            hasChanges.value = JSON.stringify(newConfig) !== originalConfig.value;
        }
    },
    { deep: true },
);

async function handleSave() {
    if (!config.value) return;
    clearMessages();
    saving.value = true;
    try {
        config.value = await updateHomepageConfig(config.value);
        originalConfig.value = JSON.stringify(config.value);
        hasChanges.value = false;
        message.value = 'Homepage settings saved';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save homepage config';
    } finally {
        saving.value = false;
    }
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
    if (!config.value) return;
    config.value.sections = config.value.sections.filter((s) => s.id !== sectionId);
}

function handleToggleSection(sectionId: string) {
    if (!config.value) return;
    const section = config.value.sections.find((s) => s.id === sectionId);
    if (section) {
        section.enabled = !section.enabled;
    }
}

function handleMoveSection(sectionId: string, direction: 'up' | 'down') {
    if (!config.value) return;
    const index = config.value.sections.findIndex((s) => s.id === sectionId);
    if (index === -1) return;

    const newIndex = direction === 'up' ? index - 1 : index + 1;
    if (newIndex < 0 || newIndex >= config.value.sections.length) return;

    // Swap sections
    const sections = [...config.value.sections];
    const temp = sections[index]!;
    sections[index] = sections[newIndex]!;
    sections[newIndex] = temp;

    // Update order values
    sections.forEach((s, i) => {
        s.order = i;
    });

    config.value.sections = sections;
}

function handleSectionSaved(section: HomepageSection) {
    if (!config.value) return;

    const existingIndex = config.value.sections.findIndex((s) => s.id === section.id);
    if (existingIndex >= 0) {
        config.value.sections[existingIndex] = section;
    } else {
        section.order = config.value.sections.length;
        config.value.sections.push(section);
    }
    showModal.value = false;
}

const enabledCount = computed(() => config.value?.sections.filter((s) => s.enabled).length ?? 0);
const totalCount = computed(() => config.value?.sections.length ?? 0);
</script>

<template>
    <div class="space-y-6">
        <!-- Messages -->
        <Transition
            enter-active-class="transition-all duration-200"
            enter-from-class="opacity-0 -translate-y-2"
            leave-active-class="transition-all duration-150"
            leave-to-class="opacity-0"
        >
            <div
                v-if="message"
                class="border-emerald/20 bg-emerald/5 flex items-center gap-2 rounded-lg border px-4
                    py-3"
            >
                <Icon name="heroicons:check-circle" size="16" class="text-emerald" />
                <span class="text-emerald text-xs">{{ message }}</span>
            </div>
            <div
                v-else-if="error"
                class="border-lava/20 bg-lava/5 flex items-center gap-2 rounded-lg border px-4 py-3"
            >
                <Icon name="heroicons:exclamation-circle" size="16" class="text-lava" />
                <span class="text-lava text-xs">{{ error }}</span>
            </div>
        </Transition>

        <!-- Loading State -->
        <div v-if="loading" class="flex flex-col items-center justify-center py-16">
            <LoadingSpinner />
            <p class="text-dim mt-3 text-xs">Loading homepage settings...</p>
        </div>

        <template v-else-if="config">
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

            <!-- Sections -->
            <div class="glass-panel overflow-hidden">
                <div class="border-border flex items-center justify-between border-b px-5 py-4">
                    <div class="flex items-center gap-3">
                        <div class="bg-lava/10 flex h-8 w-8 items-center justify-center rounded-lg">
                            <Icon name="heroicons:squares-2x2" size="16" class="text-lava" />
                        </div>
                        <div>
                            <h3 class="text-sm font-semibold text-white">Content Sections</h3>
                            <p class="text-dim text-[11px]">
                                <span v-if="totalCount > 0">
                                    {{ enabledCount }} of {{ totalCount }} sections enabled
                                </span>
                                <span v-else>Organize your homepage content</span>
                            </p>
                        </div>
                    </div>
                    <button
                        @click="handleAddSection"
                        class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-3
                            py-2 text-xs font-medium text-white transition-all hover:scale-[1.02]
                            active:scale-[0.98]"
                    >
                        <Icon name="heroicons:plus" size="14" />
                        Add Section
                    </button>
                </div>

                <!-- Empty State -->
                <div
                    v-if="config.sections.length === 0"
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
                        @click="handleAddSection"
                        class="border-border hover:border-lava/30 hover:bg-lava/5 flex items-center
                            gap-2 rounded-lg border px-4 py-2 text-xs font-medium text-white
                            transition-all"
                    >
                        <Icon name="heroicons:plus" size="14" class="text-lava" />
                        Create your first section
                    </button>
                </div>

                <!-- Section List -->
                <div v-else class="divide-border divide-y">
                    <TransitionGroup name="list" tag="div" class="divide-border divide-y">
                        <SettingsHomepageSectionItem
                            v-for="(section, index) in config.sections"
                            :key="section.id"
                            :section="section"
                            :is-first="index === 0"
                            :is-last="index === config.sections.length - 1"
                            @edit="handleEditSection"
                            @delete="handleDeleteSection"
                            @toggle="handleToggleSection"
                            @move="handleMoveSection"
                        />
                    </TransitionGroup>
                </div>
            </div>

            <!-- Save Button -->
            <div
                class="border-border bg-surface/80 sticky bottom-0 -mx-6 flex items-center
                    justify-between border-t px-6 py-4 backdrop-blur-sm"
            >
                <div class="text-dim text-xs">
                    <span v-if="hasChanges" class="text-lava flex items-center gap-1.5">
                        <span class="bg-lava h-1.5 w-1.5 animate-pulse rounded-full"></span>
                        Unsaved changes
                    </span>
                    <span v-else>All changes saved</span>
                </div>
                <button
                    @click="handleSave"
                    :disabled="saving || !hasChanges"
                    class="bg-lava hover:bg-lava-glow flex items-center gap-2 rounded-lg px-5 py-2.5
                        text-xs font-semibold text-white transition-all hover:scale-[1.02]
                        active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-40
                        disabled:hover:scale-100"
                >
                    <Icon
                        v-if="saving"
                        name="heroicons:arrow-path"
                        size="14"
                        class="animate-spin"
                    />
                    <Icon v-else name="heroicons:check" size="14" />
                    {{ saving ? 'Saving...' : 'Save Changes' }}
                </button>
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
