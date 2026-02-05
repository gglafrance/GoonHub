<script setup lang="ts">
import type { ParsingRule, ParsingPreset, ParsingRuleType } from '~/types/parsing-rules';
import { RULE_TYPE_INFO } from '~/types/parsing-rules';

const settingsStore = useSettingsStore();
const { generateId, applyRules, getBuiltInPresets } = useParsingRulesEngine();

// Local state backed by draft
const selectedPresetId = computed({
    get: () => settingsStore.draft?.parsing_rules?.activePresetId ?? null,
    set: (val) => {
        if (settingsStore.draft?.parsing_rules)
            settingsStore.draft.parsing_rules.activePresetId = val;
    },
});
const localPresets = computed({
    get: () => settingsStore.draft?.parsing_rules?.presets ?? [],
    set: (val) => {
        if (settingsStore.draft?.parsing_rules) settingsStore.draft.parsing_rules.presets = val;
    },
});
const isLoading = ref(false);
const showSaveModal = ref(false);
const showNewPresetModal = ref(false);
const showAddMenu = ref(false);
const previewInput = ref(
    'Dakota Tyler - [BaDoinkVR-VR2Normal] - [2022] - The Magic Number[x265].mkv',
);
const draggedIndex = ref<number | null>(null);
const addButtonRef = ref<HTMLElement | null>(null);
const addMenuRef = ref<HTMLElement | null>(null);
const dropdownStyle = ref<{ top: string; left: string }>({ top: '0px', left: '0px' });

// Rule type options for add menu - categorized
const ruleCategories = [
    {
        label: 'Removal',
        icon: 'heroicons:x-circle',
        types: [
            'remove_brackets',
            'remove_numbers',
            'remove_years',
            'remove_special_chars',
            'remove_stopwords',
            'remove_duplicates',
        ] as ParsingRuleType[],
    },
    {
        label: 'Transform',
        icon: 'heroicons:arrow-path',
        types: ['regex_remove', 'text_replace'] as ParsingRuleType[],
    },
    {
        label: 'Filter',
        icon: 'heroicons:funnel',
        types: ['word_length_filter', 'case_normalize'] as ParsingRuleType[],
    },
];

// Computed
const currentPreset = computed(() => {
    if (!selectedPresetId.value) return null;
    return localPresets.value.find((p) => p.id === selectedPresetId.value) || null;
});

const previewOutput = computed(() => {
    if (!currentPreset.value || !previewInput.value) return previewInput.value;
    return applyRules(previewInput.value, currentPreset.value.rules);
});

const enabledRulesCount = computed(() => {
    if (!currentPreset.value) return 0;
    return currentPreset.value.rules.filter((r) => r.enabled).length;
});

const totalRulesCount = computed(() => {
    if (!currentPreset.value) return 0;
    return currentPreset.value.rules.length;
});

// Dropdown positioning
function updateDropdownPosition() {
    if (!addButtonRef.value) return;
    const rect = addButtonRef.value.getBoundingClientRect();
    dropdownStyle.value = {
        top: `${rect.bottom + 8}px`,
        left: `${rect.right - 224}px`, // 224px = w-56 (14rem)
    };
}

// Close add menu when clicking outside
function handleClickOutside(event: MouseEvent) {
    const target = event.target as Node;
    if (
        addMenuRef.value &&
        !addMenuRef.value.contains(target) &&
        addButtonRef.value &&
        !addButtonRef.value.contains(target)
    ) {
        showAddMenu.value = false;
    }
}

// Toggle dropdown with position update
function toggleAddMenu() {
    if (!showAddMenu.value) {
        updateDropdownPosition();
    }
    showAddMenu.value = !showAddMenu.value;
}

watch(
    () => showAddMenu.value,
    (open) => {
        if (open) {
            updateDropdownPosition();
            setTimeout(() => document.addEventListener('click', handleClickOutside), 0);
            window.addEventListener('scroll', updateDropdownPosition, true);
            window.addEventListener('resize', updateDropdownPosition);
        } else {
            document.removeEventListener('click', handleClickOutside);
            window.removeEventListener('scroll', updateDropdownPosition, true);
            window.removeEventListener('resize', updateDropdownPosition);
        }
    },
);

onBeforeUnmount(() => {
    document.removeEventListener('click', handleClickOutside);
    window.removeEventListener('scroll', updateDropdownPosition, true);
    window.removeEventListener('resize', updateDropdownPosition);
});

// Data comes from draft - no separate loading needed

// Select a preset
function selectPreset(presetId: string | null) {
    selectedPresetId.value = presetId;

    // If selecting a built-in preset, copy it to local presets if not already there
    if (presetId) {
        const builtIn = getBuiltInPresets().find((p) => p.id === presetId);
        if (builtIn && !localPresets.value.find((p) => p.id === presetId)) {
            localPresets.value.push(JSON.parse(JSON.stringify(builtIn)));
        }
    }
}

// Add a new rule
function addRule(type: ParsingRuleType) {
    if (!currentPreset.value) return;

    const newRule: ParsingRule = {
        id: generateId(),
        type,
        enabled: true,
        order: currentPreset.value.rules.length,
        config: type === 'case_normalize' ? { caseType: 'lower' } : {},
    };

    const preset = localPresets.value.find((p) => p.id === currentPreset.value!.id);
    if (preset) {
        preset.rules.push(newRule);
    }

    showAddMenu.value = false;
}

// Update a rule
function updateRule(ruleId: string, updated: ParsingRule) {
    if (!currentPreset.value) return;

    const preset = localPresets.value.find((p) => p.id === currentPreset.value!.id);
    if (!preset) return;

    const ruleIndex = preset.rules.findIndex((r) => r.id === ruleId);
    if (ruleIndex !== -1) {
        preset.rules[ruleIndex] = updated;
    }
}

// Toggle rule enabled state
function toggleRule(ruleId: string) {
    if (!currentPreset.value) return;

    const presetIndex = localPresets.value.findIndex((p) => p.id === currentPreset.value!.id);
    if (presetIndex === -1) return;

    const preset = localPresets.value[presetIndex];
    if (!preset) return;

    const ruleIndex = preset.rules.findIndex((r) => r.id === ruleId);
    const rule = preset.rules[ruleIndex];
    if (ruleIndex !== -1 && rule) {
        rule.enabled = !rule.enabled;
    }
}

// Delete a rule
function deleteRule(ruleId: string) {
    if (!currentPreset.value) return;

    const presetIndex = localPresets.value.findIndex((p) => p.id === currentPreset.value!.id);
    if (presetIndex === -1) return;

    const preset = localPresets.value[presetIndex];
    if (!preset) return;

    preset.rules = preset.rules.filter((r) => r.id !== ruleId);

    // Update order values
    preset.rules.forEach((r, i) => {
        r.order = i;
    });
}

// Drag and drop
function handleDragStart(index: number) {
    draggedIndex.value = index;
}

function handleDragOver(e: DragEvent, index: number) {
    e.preventDefault();
    if (draggedIndex.value === null || draggedIndex.value === index) return;

    if (!currentPreset.value) return;

    const presetIndex = localPresets.value.findIndex((p) => p.id === currentPreset.value!.id);
    if (presetIndex === -1) return;

    const preset = localPresets.value[presetIndex];
    if (!preset) return;

    const rules = [...preset.rules];
    const draggedRule = rules[draggedIndex.value];
    if (!draggedRule) return;

    rules.splice(draggedIndex.value, 1);
    rules.splice(index, 0, draggedRule);

    // Update order values
    rules.forEach((r, i) => {
        r.order = i;
    });

    preset.rules = rules;
    draggedIndex.value = index;
}

function handleDragEnd() {
    draggedIndex.value = null;
}

// Create a new empty preset
function handleCreateNewPreset(name: string) {
    const newPreset: ParsingPreset = {
        id: generateId(),
        name,
        isBuiltIn: false,
        rules: [],
    };

    localPresets.value.push(newPreset);
    selectedPresetId.value = newPreset.id;
    showNewPresetModal.value = false;
}

// Save as new preset (copy current rules)
function handleSaveAsPreset(name: string) {
    const newPreset: ParsingPreset = {
        id: generateId(),
        name,
        isBuiltIn: false,
        rules: currentPreset.value ? JSON.parse(JSON.stringify(currentPreset.value.rules)) : [],
    };

    localPresets.value.push(newPreset);
    selectedPresetId.value = newPreset.id;
    showSaveModal.value = false;
}

// Delete preset
function deletePreset() {
    if (!currentPreset.value || currentPreset.value.isBuiltIn) return;

    localPresets.value = localPresets.value.filter((p) => p.id !== currentPreset.value!.id);
    selectedPresetId.value = null;
}
</script>

<template>
    <div class="space-y-5">
        <!-- Loading state -->
        <div
            v-if="isLoading && !settingsStore.draft?.parsing_rules"
            class="flex justify-center py-16"
        >
            <div class="text-center">
                <div class="relative mx-auto h-10 w-10">
                    <div class="bg-lava/20 absolute inset-0 animate-ping rounded-full"></div>
                    <div
                        class="bg-lava/10 relative flex h-10 w-10 items-center justify-center
                            rounded-full"
                    >
                        <Icon name="heroicons:funnel" size="20" class="text-lava animate-pulse" />
                    </div>
                </div>
                <p class="text-dim mt-4 text-xs">Loading parsing rules...</p>
            </div>
        </div>

        <template v-else>
            <!-- Header section with info -->
            <div class="glass-panel overflow-hidden">
                <div class="relative p-5">
                    <!-- Decorative gradient -->
                    <div
                        class="bg-lava/5 absolute top-0 right-0 -mt-8 -mr-8 h-32 w-32 rounded-full
                            blur-3xl"
                    ></div>

                    <div class="relative flex items-start gap-4">
                        <div
                            class="from-lava/20 to-lava/5 ring-lava/20 flex h-11 w-11 shrink-0
                                items-center justify-center rounded-xl bg-linear-to-br ring-1"
                        >
                            <Icon name="heroicons:funnel" size="22" class="text-lava" />
                        </div>
                        <div>
                            <h3 class="text-sm font-semibold text-white">Filename Parsing Rules</h3>
                            <p class="text-dim mt-1 max-w-lg text-xs leading-relaxed">
                                Configure rules to clean video filenames before searching ThePornDB.
                                Rules are applied in order from top to bottom.
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Preset selector and controls -->
            <div class="glass-panel p-5">
                <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
                    <div class="flex-1">
                        <label
                            class="text-dim mb-2 flex items-center gap-1.5 text-[11px] font-medium
                                tracking-wider uppercase"
                        >
                            <Icon name="heroicons:bookmark" size="12" />
                            Active Preset
                        </label>
                        <div class="relative">
                            <select
                                v-model="selectedPresetId"
                                class="border-border bg-void/80 focus:border-lava/40
                                    focus:ring-lava/20 w-full appearance-none rounded-lg border
                                    py-2.5 pr-10 pl-4 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                                @change="selectPreset(selectedPresetId)"
                            >
                                <option :value="null">No parsing rules</option>
                                <optgroup label="Built-in Presets">
                                    <option
                                        v-for="preset in getBuiltInPresets()"
                                        :key="preset.id"
                                        :value="preset.id"
                                    >
                                        {{ preset.name }}
                                    </option>
                                </optgroup>
                                <optgroup
                                    v-if="localPresets.filter((p) => !p.isBuiltIn).length > 0"
                                    label="Custom Presets"
                                >
                                    <option
                                        v-for="preset in localPresets.filter((p) => !p.isBuiltIn)"
                                        :key="preset.id"
                                        :value="preset.id"
                                    >
                                        {{ preset.name }}
                                    </option>
                                </optgroup>
                            </select>
                            <Icon
                                name="heroicons:chevron-up-down"
                                size="16"
                                class="text-dim pointer-events-none absolute top-1/2 right-3
                                    -translate-y-1/2"
                            />
                        </div>
                    </div>

                    <div class="flex gap-2">
                        <button
                            class="group border-border hover:border-lava/40 hover:bg-lava/5 flex
                                items-center gap-1.5 rounded-lg border px-3.5 py-3 text-xs
                                font-medium text-white transition-all"
                            @click="showNewPresetModal = true"
                        >
                            <Icon
                                name="heroicons:plus"
                                size="14"
                                class="text-dim group-hover:text-lava transition-colors"
                            />
                            <span class="hidden sm:inline">New Preset</span>
                        </button>
                        <button
                            :disabled="!currentPreset"
                            class="border-border hover:border-lava/40 hover:bg-lava/5
                                disabled:hover:border-border flex items-center gap-1.5 rounded-lg
                                border px-3.5 py-3 text-xs font-medium text-white transition-all
                                disabled:cursor-not-allowed disabled:opacity-40
                                disabled:hover:bg-transparent"
                            @click="showSaveModal = true"
                        >
                            <Icon name="heroicons:document-duplicate" size="14" class="text-dim" />
                            <span class="hidden sm:inline">Save As</span>
                        </button>
                        <button
                            v-if="currentPreset && !currentPreset.isBuiltIn"
                            class="border-border flex items-center gap-1.5 rounded-lg border px-3.5
                                py-3 text-xs font-medium text-white transition-all
                                hover:border-red-500/40 hover:bg-red-500/5 hover:text-red-400"
                            @click="deletePreset"
                        >
                            <Icon name="heroicons:trash" size="14" />
                        </button>
                    </div>
                </div>
            </div>

            <!-- Rules list -->
            <div v-if="currentPreset" class="glass-panel overflow-hidden">
                <!-- Rules header -->
                <div class="border-border flex items-center justify-between border-b px-5 py-3">
                    <div class="flex items-center gap-3">
                        <h4 class="text-sm font-semibold text-white">Rules Pipeline</h4>
                        <div v-if="totalRulesCount > 0" class="flex items-center gap-1.5">
                            <span
                                class="bg-lava/10 text-lava rounded-full px-2 py-0.5 text-[10px]
                                    font-medium"
                            >
                                {{ enabledRulesCount }} / {{ totalRulesCount }} active
                            </span>
                        </div>
                    </div>
                    <button
                        ref="addButtonRef"
                        class="group border-lava/30 bg-lava/5 text-lava hover:border-lava/50
                            hover:bg-lava/10 flex items-center gap-1.5 rounded-lg border px-3 py-1.5
                            text-xs font-medium transition-all"
                        @click="toggleAddMenu"
                    >
                        <Icon name="heroicons:plus" size="14" />
                        Add Rule
                        <Icon
                            name="heroicons:chevron-down"
                            size="12"
                            class="transition-transform duration-200"
                            :class="showAddMenu ? 'rotate-180' : ''"
                        />
                    </button>

                    <!-- Add menu dropdown (teleported to body) -->
                    <Teleport to="body">
                        <Transition
                            enter-active-class="transition-all duration-200 ease-out"
                            enter-from-class="opacity-0 scale-95 -translate-y-1"
                            enter-to-class="opacity-100 scale-100 translate-y-0"
                            leave-active-class="transition-all duration-150 ease-in"
                            leave-from-class="opacity-100 scale-100 translate-y-0"
                            leave-to-class="opacity-0 scale-95 -translate-y-1"
                        >
                            <div
                                v-if="showAddMenu"
                                ref="addMenuRef"
                                class="border-border bg-panel fixed z-9999 w-56 origin-top-right
                                    rounded-xl border p-1.5 shadow-2xl"
                                :style="dropdownStyle"
                            >
                                <div
                                    v-for="category in ruleCategories"
                                    :key="category.label"
                                    class="mb-1 last:mb-0"
                                >
                                    <div
                                        class="text-dim flex items-center gap-1.5 px-2 py-1.5
                                            text-[10px] font-medium tracking-wider uppercase"
                                    >
                                        <Icon :name="category.icon" size="11" />
                                        {{ category.label }}
                                    </div>
                                    <button
                                        v-for="type in category.types"
                                        :key="type"
                                        class="group hover:bg-lava/10 flex w-full items-center
                                            justify-between rounded-lg px-2.5 py-2 text-left text-xs
                                            transition-all"
                                        @click="addRule(type)"
                                    >
                                        <span class="text-white/80 group-hover:text-white">
                                            {{ RULE_TYPE_INFO[type].label }}
                                        </span>
                                        <Icon
                                            name="heroicons:plus-circle"
                                            size="14"
                                            class="text-dim group-hover:text-lava opacity-0
                                                transition-all group-hover:opacity-100"
                                        />
                                    </button>
                                </div>
                            </div>
                        </Transition>
                    </Teleport>
                </div>

                <!-- Empty state -->
                <div
                    v-if="currentPreset.rules.length === 0"
                    class="flex flex-col items-center justify-center py-16"
                >
                    <div class="relative">
                        <div
                            class="bg-lava/10 absolute inset-0 animate-pulse rounded-full blur-xl"
                        ></div>
                        <div
                            class="bg-void/50 relative flex h-16 w-16 items-center justify-center
                                rounded-2xl border border-dashed border-white/10"
                        >
                            <Icon name="heroicons:funnel" size="28" class="text-dim" />
                        </div>
                    </div>
                    <p class="text-dim mt-4 text-sm font-medium">No rules configured</p>
                    <p class="text-dim/60 mt-1 text-xs">
                        Click "Add Rule" to start building your pipeline
                    </p>
                </div>

                <!-- Rules list -->
                <div v-else class="divide-border/50 divide-y">
                    <div
                        v-for="(rule, index) in currentPreset.rules"
                        :key="rule.id"
                        draggable="true"
                        class="rule-item"
                        @dragstart="handleDragStart(index)"
                        @dragover="(e) => handleDragOver(e, index)"
                        @dragend="handleDragEnd"
                    >
                        <SettingsParsingRulesRuleRow
                            :rule="rule"
                            :index="index"
                            :is-dragging="draggedIndex === index"
                            @update="updateRule(rule.id, $event)"
                            @delete="deleteRule(rule.id)"
                            @toggle="toggleRule(rule.id)"
                        />
                    </div>
                </div>
            </div>

            <!-- Preview panel -->
            <div v-if="currentPreset" class="glass-panel overflow-hidden">
                <div class="border-border border-b px-5 py-3">
                    <div class="flex items-center gap-2">
                        <Icon name="heroicons:eye" size="16" class="text-lava" />
                        <h4 class="text-sm font-semibold text-white">Live Preview</h4>
                    </div>
                </div>
                <div class="p-5">
                    <div class="space-y-4">
                        <!-- Input -->
                        <div>
                            <label
                                class="text-dim mb-2 flex items-center gap-1.5 text-[11px]
                                    font-medium tracking-wider uppercase"
                            >
                                <Icon name="heroicons:arrow-right-circle" size="12" />
                                Input Filename
                            </label>
                            <input
                                v-model="previewInput"
                                type="text"
                                placeholder="Enter a filename to test..."
                                class="border-border bg-void/80 placeholder:text-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-4 py-3 font-mono text-xs text-white transition-all
                                    focus:ring-1 focus:outline-none"
                            />
                        </div>

                        <!-- Transformation indicator -->
                        <div class="flex items-center justify-center gap-2 py-1">
                            <div
                                class="via-border h-px flex-1 bg-linear-to-r from-transparent
                                    to-transparent"
                            ></div>
                            <div
                                class="bg-lava/10 flex items-center gap-1.5 rounded-full px-3 py-1"
                            >
                                <Icon name="heroicons:arrow-down" size="12" class="text-lava" />
                                <span class="text-lava text-[10px] font-medium">
                                    {{ enabledRulesCount }} rule{{
                                        enabledRulesCount !== 1 ? 's' : ''
                                    }}
                                    applied
                                </span>
                            </div>
                            <div
                                class="via-border h-px flex-1 bg-linear-to-r from-transparent
                                    to-transparent"
                            ></div>
                        </div>

                        <!-- Output -->
                        <div>
                            <label
                                class="text-dim mb-2 flex items-center gap-1.5 text-[11px]
                                    font-medium tracking-wider uppercase"
                            >
                                <Icon
                                    name="heroicons:check-circle"
                                    size="12"
                                    class="text-emerald-400"
                                />
                                Cleaned Output
                            </label>
                            <div
                                class="relative overflow-hidden rounded-lg border
                                    border-emerald-500/20 bg-emerald-500/5 px-4 py-3"
                            >
                                <code class="block font-mono text-xs text-emerald-400">
                                    {{ previewOutput || '(empty result)' }}
                                </code>
                                <!-- Success indicator -->
                                <div
                                    v-if="
                                        previewOutput &&
                                        previewOutput !== previewInput.replace(/\.[^/.]+$/, '')
                                    "
                                    class="absolute top-3 right-3"
                                >
                                    <Icon
                                        name="heroicons:sparkles"
                                        size="14"
                                        class="text-emerald-400/50"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </template>

        <!-- Save As modal -->
        <SettingsParsingRulesPresetSaveModal
            :visible="showSaveModal"
            title="Duplicate Preset"
            description="Create a copy of the current preset with a new name."
            @close="showSaveModal = false"
            @save="handleSaveAsPreset"
        />

        <!-- New Preset modal -->
        <SettingsParsingRulesPresetSaveModal
            :visible="showNewPresetModal"
            title="Create New Preset"
            description="Start fresh with an empty preset."
            @close="showNewPresetModal = false"
            @save="handleCreateNewPreset"
        />
    </div>
</template>

<style scoped></style>
