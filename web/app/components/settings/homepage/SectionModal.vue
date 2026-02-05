<script setup lang="ts">
import type { HomepageSection, SectionType } from '~/types/homepage';
import {
    SECTION_TYPE_LABELS,
    SORT_OPTIONS,
    SECTION_SORT_OPTIONS,
    SECTION_ICONS,
    SECTION_COLORS_WITH_BORDER,
    SECTION_DESCRIPTIONS,
} from '~/types/homepage';

const props = defineProps<{
    section: HomepageSection | null;
}>();

const emit = defineEmits<{
    close: [];
    save: [section: HomepageSection];
}>();

const isEditing = computed(() => !!props.section);

// Form state
const id = ref('');
const type = ref<SectionType>('latest');
const title = ref(SECTION_TYPE_LABELS.latest);
const enabled = ref(true);
const limit = ref(12);
const sort = ref('created_at_desc');
const config = ref<Record<string, unknown>>({});

// Initialize form and setup escape key handler
onMounted(() => {
    // Initialize form from section if editing
    if (props.section) {
        id.value = props.section.id;
        type.value = props.section.type as SectionType;
        title.value = props.section.title;
        enabled.value = props.section.enabled;
        limit.value = props.section.limit;
        sort.value = props.section.sort || 'created_at_desc';
        config.value = { ...props.section.config };
    } else {
        // Generate a new ID for new sections
        id.value = `section-${Date.now()}`;
    }

    // Handle escape key
    const handleEscape = (e: KeyboardEvent) => {
        if (e.key === 'Escape') handleClose();
    };
    window.addEventListener('keydown', handleEscape);
    onUnmounted(() => window.removeEventListener('keydown', handleEscape));
});

// Section type options with icons and descriptions
const typeOptions = computed(() => {
    return Object.entries(SECTION_TYPE_LABELS).map(([value, label]) => ({
        value: value as SectionType,
        label,
        icon: SECTION_ICONS[value as SectionType] || 'heroicons:squares-2x2',
        color:
            SECTION_COLORS_WITH_BORDER[value as SectionType] ||
            'text-dim bg-white/5 border-white/10',
        description: SECTION_DESCRIPTIONS[value as SectionType] || '',
    }));
});

const requiresConfig = computed(() => {
    return ['actor', 'studio', 'tag', 'saved_search'].includes(type.value);
});

// Check if required config fields are present
const isConfigValid = computed(() => {
    switch (type.value) {
        case 'actor':
            return !!config.value.actor_uuid;
        case 'studio':
            return !!config.value.studio_uuid;
        case 'tag':
            return !!config.value.tag_id;
        case 'saved_search':
            return !!config.value.saved_search_uuid;
        default:
            return true;
    }
});

// Get available sort options for current section type
const availableSortOptions = computed(() => {
    const options = SECTION_SORT_OPTIONS[type.value];
    return options.length > 0 ? options : SORT_OPTIONS;
});

// Whether sorting is available for current section type
const canSort = computed(() => {
    return SECTION_SORT_OPTIONS[type.value].length !== 0;
});

// Current type option for display
const currentTypeOption = computed(() => {
    return typeOptions.value.find((o) => o.value === type.value);
});

// When type changes, update title, reset config, and adjust sort options
watch(type, (newType) => {
    // Update title and config for new sections
    if (!props.section) {
        title.value = SECTION_TYPE_LABELS[newType] || newType;
        config.value = {};
    }

    // Reset sort to appropriate default for the new type
    const options = SECTION_SORT_OPTIONS[newType];
    if (options.length === 0) {
        sort.value = '';
    } else if (!props.section) {
        // New sections: use first option as default
        sort.value = options[0]?.value ?? 'created_at_desc';
    } else if (!options.find((o) => o.value === sort.value)) {
        // Editing: only reset if current sort is not available
        sort.value = options[0]?.value ?? 'created_at_desc';
    }
});

function handleSave() {
    const section: HomepageSection = {
        id: id.value,
        type: type.value,
        title: title.value,
        enabled: enabled.value,
        limit: limit.value,
        order: props.section?.order ?? 0,
        sort: sort.value,
        config: config.value,
    };
    emit('save', section);
}

function handleClose() {
    emit('close');
}
</script>

<template>
    <Teleport to="body">
        <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
            <!-- Backdrop -->
            <div class="bg-void/80 absolute inset-0 backdrop-blur-sm" @click="handleClose"></div>

            <!-- Modal -->
            <Transition
                appear
                enter-active-class="transition-all duration-200"
                enter-from-class="opacity-0 scale-95"
            >
                <div
                    class="bg-surface border-border relative z-10 w-full max-w-lg rounded-xl border
                        shadow-2xl"
                >
                    <!-- Header -->
                    <div class="border-border flex items-center justify-between border-b px-6 py-4">
                        <div class="flex items-center gap-3">
                            <div
                                v-if="currentTypeOption"
                                class="flex h-8 w-8 items-center justify-center rounded-lg border"
                                :class="currentTypeOption.color"
                            >
                                <Icon :name="currentTypeOption.icon" size="16" />
                            </div>
                            <div>
                                <h2 class="text-sm font-semibold text-white">
                                    {{ isEditing ? 'Edit Section' : 'Add Section' }}
                                </h2>
                                <p class="text-dim text-[11px]">
                                    {{
                                        isEditing
                                            ? 'Modify section settings'
                                            : 'Configure a new homepage section'
                                    }}
                                </p>
                            </div>
                        </div>
                        <button
                            class="text-dim -m-2 flex items-center justify-center rounded-lg p-2
                                transition-all hover:bg-white/10 hover:text-white"
                            @click="handleClose"
                        >
                            <Icon name="heroicons:x-mark" size="20" />
                        </button>
                    </div>

                    <!-- Body -->
                    <div class="max-h-[60vh] space-y-5 overflow-y-auto p-6">
                        <!-- Section Type Selection (only for new sections) -->
                        <div v-if="!isEditing">
                            <label
                                class="text-dim mb-2 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Section Type
                            </label>
                            <div class="grid grid-cols-2 gap-2">
                                <button
                                    v-for="option in typeOptions"
                                    :key="option.value"
                                    class="flex items-start gap-3 rounded-lg border p-3 text-left
                                        transition-all"
                                    :class="
                                        type === option.value
                                            ? 'border-lava/40 bg-lava/5'
                                            : 'border-border hover:border-white/20 hover:bg-white/2'
                                    "
                                    @click="type = option.value"
                                >
                                    <div
                                        class="mt-0.5 flex h-7 w-7 shrink-0 items-center
                                            justify-center rounded-md border"
                                        :class="option.color"
                                    >
                                        <Icon :name="option.icon" size="14" />
                                    </div>
                                    <div class="min-w-0">
                                        <div
                                            class="text-xs font-medium"
                                            :class="
                                                type === option.value
                                                    ? 'text-white'
                                                    : 'text-white/80'
                                            "
                                        >
                                            {{ option.label }}
                                        </div>
                                        <div class="text-dim mt-0.5 line-clamp-2 text-[10px]">
                                            {{ option.description }}
                                        </div>
                                    </div>
                                </button>
                            </div>
                        </div>

                        <!-- Section Details -->
                        <div class="border-border space-y-4 rounded-lg border p-4">
                            <h3
                                class="text-dim -mt-1 text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Section Details
                            </h3>

                            <!-- Title -->
                            <div>
                                <label
                                    class="text-dim mb-1.5 block text-[11px] font-medium
                                        tracking-wider uppercase"
                                >
                                    Display Title
                                </label>
                                <input
                                    v-model="title"
                                    type="text"
                                    maxlength="100"
                                    placeholder="Enter section title..."
                                    class="border-border bg-void/80 focus:border-lava/40
                                        focus:ring-lava/20 placeholder:text-dim/50 w-full rounded-lg
                                        border px-3.5 py-2.5 text-sm text-white transition-all
                                        focus:ring-1 focus:outline-none"
                                />
                            </div>

                            <!-- Limit & Sort Row -->
                            <div class="flex gap-4">
                                <!-- Limit -->
                                <div class="w-28">
                                    <label
                                        class="text-dim mb-1.5 block text-[11px] font-medium
                                            tracking-wider uppercase"
                                    >
                                        Show
                                    </label>
                                    <div class="relative">
                                        <input
                                            v-model.number="limit"
                                            type="number"
                                            min="1"
                                            max="50"
                                            class="border-border bg-void/80 focus:border-lava/40
                                                focus:ring-lava/20 w-full rounded-lg border px-3.5
                                                py-2.5 pr-12 text-sm text-white transition-all
                                                focus:ring-1 focus:outline-none"
                                        />
                                        <span
                                            class="text-dim pointer-events-none absolute top-1/2
                                                right-3 -translate-y-1/2 text-xs"
                                            >items</span
                                        >
                                    </div>
                                </div>

                                <!-- Sort -->
                                <div v-if="canSort" class="flex-1">
                                    <label
                                        class="text-dim mb-1.5 block text-[11px] font-medium
                                            tracking-wider uppercase"
                                    >
                                        Sort Order
                                    </label>
                                    <UiSelectMenu v-model="sort" :options="availableSortOptions" />
                                </div>
                            </div>
                        </div>

                        <!-- Type-specific config -->
                        <div
                            v-if="requiresConfig"
                            class="border-border space-y-4 rounded-lg border p-4"
                        >
                            <h3
                                class="text-dim -mt-1 text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Content Source
                            </h3>

                            <SettingsHomepageActorConfig v-if="type === 'actor'" v-model="config" />
                            <SettingsHomepageStudioConfig
                                v-if="type === 'studio'"
                                v-model="config"
                            />
                            <SettingsHomepageTagConfig v-if="type === 'tag'" v-model="config" />
                            <SettingsHomepageSavedSearchConfig
                                v-if="type === 'saved_search'"
                                v-model="config"
                            />

                            <!-- Validation Message -->
                            <div
                                v-if="!isConfigValid"
                                class="bg-lava/5 border-lava/20 flex items-center gap-2 rounded-lg
                                    border px-3 py-2"
                            >
                                <Icon
                                    name="heroicons:exclamation-triangle"
                                    size="14"
                                    class="text-lava"
                                />
                                <span class="text-lava text-xs">
                                    Please select a {{ type.replace('_', ' ') }} to continue.
                                </span>
                            </div>
                        </div>

                        <!-- Visibility Toggle -->
                        <div class="border-border rounded-lg border p-4">
                            <div class="flex items-center justify-between">
                                <div>
                                    <span class="block text-xs font-medium text-white"
                                        >Section Enabled</span
                                    >
                                    <span class="text-dim text-[11px]"
                                        >Show this section on the homepage</span
                                    >
                                </div>
                                <UiToggle v-model="enabled" />
                            </div>
                        </div>
                    </div>

                    <!-- Footer -->
                    <div
                        class="border-border flex items-center justify-end gap-3 border-t px-6 py-4"
                    >
                        <button
                            class="border-border rounded-lg border px-4 py-2.5 text-xs font-medium
                                text-white transition-colors hover:bg-white/5"
                            @click="handleClose"
                        >
                            Cancel
                        </button>
                        <button
                            :disabled="!title.trim() || !isConfigValid"
                            class="bg-lava hover:bg-lava-glow flex items-center gap-2 rounded-lg
                                px-5 py-2.5 text-xs font-semibold text-white transition-all
                                hover:scale-[1.02] active:scale-[0.98] disabled:cursor-not-allowed
                                disabled:opacity-40 disabled:hover:scale-100"
                            @click="handleSave"
                        >
                            <Icon name="heroicons:check" size="14" />
                            {{ isEditing ? 'Save Changes' : 'Add Section' }}
                        </button>
                    </div>
                </div>
            </Transition>
        </div>
    </Teleport>
</template>
