<script setup lang="ts">
import type { Actor, UpdateActorInput } from '~/types/actor';
import type { PornDBPerformer, PornDBPerformerDetails } from '~/types/porndb';

const props = defineProps<{
    visible: boolean;
    actorName: string;
    currentActor: Actor;
}>();

const emit = defineEmits<{
    close: [];
    apply: [data: Partial<UpdateActorInput>];
}>();

const apiPornDB = useApiPornDB();

// Use the fetch metadata composable for race condition handling
const {
    searchQuery,
    isSearching,
    searchResults,
    searchError,
    selectedItem: selectedPerformer,
    isFetchingDetails,
    itemDetails: performerDetails,
    detailsError,
    prefetchingId,
    search: searchPerformers,
    fetchDetails: fetchPerformerDetails,
    handleHover,
    handleHoverLeave,
    goBack,
    cleanup,
} = useFetchMetadata<PornDBPerformer, PornDBPerformerDetails>({
    searchFn: apiPornDB.searchPornDBPerformers,
    fetchDetailsFn: apiPornDB.getPornDBPerformerWithFallback,
    getItemId: (p) => p.id,
});

// Get the selected performer's ID for template comparison (type-safe)
const selectedPerformerId = computed(() => selectedPerformer.value?.id ?? null);

// Field selection for applying metadata
const selectedFields = ref<Record<string, boolean>>({});

// Field definitions for display
const fieldDefinitions: {
    key: keyof PornDBPerformerDetails;
    actorKey: keyof Actor;
    label: string;
    format?: (val: unknown) => string;
}[] = [
    { key: 'name', actorKey: 'name', label: 'Name' },
    {
        key: 'aliases',
        actorKey: 'aliases',
        label: 'Aliases',
        format: (val) => (val as string[])?.join(', ') || '-',
    },
    { key: 'image', actorKey: 'image_url', label: 'Image' },
    { key: 'gender', actorKey: 'gender', label: 'Gender' },
    {
        key: 'birthday',
        actorKey: 'birthday',
        label: 'Birthday',
        format: (val) => (val ? new Date(val as string).toLocaleDateString() : '-'),
    },
    {
        key: 'deathday',
        actorKey: 'date_of_death',
        label: 'Date of Death',
        format: (val) => (val ? new Date(val as string).toLocaleDateString() : '-'),
    },
    { key: 'astrology', actorKey: 'astrology', label: 'Astrology' },
    { key: 'birthplace', actorKey: 'birthplace', label: 'Birthplace' },
    { key: 'ethnicity', actorKey: 'ethnicity', label: 'Ethnicity' },
    { key: 'nationality', actorKey: 'nationality', label: 'Nationality' },
    { key: 'career_start_year', actorKey: 'career_start_year', label: 'Career Start' },
    { key: 'career_end_year', actorKey: 'career_end_year', label: 'Career End' },
    {
        key: 'height',
        actorKey: 'height_cm',
        label: 'Height',
        format: (val) => (val ? `${val}cm` : '-'),
    },
    {
        key: 'weight',
        actorKey: 'weight_kg',
        label: 'Weight',
        format: (val) => (val ? `${val}kg` : '-'),
    },
    { key: 'measurements', actorKey: 'measurements', label: 'Measurements' },
    { key: 'cupsize', actorKey: 'cupsize', label: 'Cup Size' },
    { key: 'hair_colour', actorKey: 'hair_color', label: 'Hair Color' },
    { key: 'eye_colour', actorKey: 'eye_color', label: 'Eye Color' },
    { key: 'tattoos', actorKey: 'tattoos', label: 'Tattoos' },
    { key: 'piercings', actorKey: 'piercings', label: 'Piercings' },
    {
        key: 'fake_boobs',
        actorKey: 'fake_boobs',
        label: 'Enhanced',
        format: (val) => (val ? 'Yes' : 'No'),
    },
    {
        key: 'same_sex_only',
        actorKey: 'same_sex_only',
        label: 'Same-sex Only',
        format: (val) => (val ? 'Yes' : 'No'),
    },
];

// Wrapper to trigger search and field selection on details load
const handleFetchPerformerDetails = (performer: PornDBPerformer) => {
    selectedFields.value = {};
    fetchPerformerDetails(performer);
};

// Initialize field selection - pre-check fields where we have new data and current is empty
const initializeFieldSelection = () => {
    if (!performerDetails.value) return;

    for (const field of fieldDefinitions) {
        const porndbValue = performerDetails.value[field.key];
        const currentValue = props.currentActor[field.actorKey];

        // Pre-select if PornDB has data and current value is empty/null/undefined
        const hasNewData = Array.isArray(porndbValue)
            ? porndbValue.length > 0
            : porndbValue !== null && porndbValue !== undefined && porndbValue !== '';
        const currentIsEmpty = Array.isArray(currentValue)
            ? currentValue.length === 0
            : currentValue === null || currentValue === undefined || currentValue === '';

        selectedFields.value[field.key] = hasNewData && currentIsEmpty;
    }
};

// Format value for display
const formatValue = (value: unknown, format?: (val: unknown) => string): string => {
    if (value === null || value === undefined || value === '') return '-';
    if (format) return format(value);
    return String(value);
};

// Get current actor value for a field
const getCurrentValue = (actorKey: keyof Actor): unknown => {
    return props.currentActor[actorKey];
};

// Get PornDB value for a field
const getPornDBValue = (key: keyof PornDBPerformerDetails): unknown => {
    return performerDetails.value?.[key];
};

// Check if a field has changed
const hasFieldChanged = (field: (typeof fieldDefinitions)[0]): boolean => {
    const porndbValue = getPornDBValue(field.key);
    const currentValue = getCurrentValue(field.actorKey);

    // Handle array comparison (e.g. aliases)
    if (Array.isArray(porndbValue) || Array.isArray(currentValue)) {
        const pArr = (porndbValue as string[]) || [];
        const cArr = (currentValue as string[]) || [];
        if (pArr.length === 0) return false;
        if (pArr.length !== cArr.length) return true;
        return pArr.some((v, i) => v !== cArr[i]);
    }

    // Normalize values for comparison
    const normalizedPorndb =
        porndbValue === null || porndbValue === undefined || porndbValue === ''
            ? null
            : porndbValue;
    const normalizedCurrent =
        currentValue === null || currentValue === undefined || currentValue === ''
            ? null
            : currentValue;

    return normalizedPorndb !== normalizedCurrent && normalizedPorndb !== null;
};

// Count selected fields
const selectedFieldCount = computed(() => {
    return Object.values(selectedFields.value).filter(Boolean).length;
});

// Apply selected metadata
const applyMetadata = () => {
    if (!performerDetails.value) return;

    const data: Partial<UpdateActorInput> = {};

    for (const field of fieldDefinitions) {
        if (selectedFields.value[field.key]) {
            const value = performerDetails.value[field.key];
            if (value !== null && value !== undefined && value !== '') {
                // Map PornDB field to actor field
                switch (field.key) {
                    case 'name':
                        data.name = value as string;
                        break;
                    case 'aliases':
                        data.aliases = value as string[];
                        break;
                    case 'image':
                        data.image_url = value as string;
                        break;
                    case 'gender':
                        data.gender = value as string;
                        break;
                    case 'birthday':
                        data.birthday = value as string;
                        break;
                    case 'deathday':
                        data.date_of_death = value as string;
                        break;
                    case 'astrology':
                        data.astrology = value as string;
                        break;
                    case 'birthplace':
                        data.birthplace = value as string;
                        break;
                    case 'ethnicity':
                        data.ethnicity = value as string;
                        break;
                    case 'nationality':
                        data.nationality = value as string;
                        break;
                    case 'career_start_year':
                        data.career_start_year = value as number;
                        break;
                    case 'career_end_year':
                        data.career_end_year = value as number;
                        break;
                    case 'height':
                        data.height_cm = value as number;
                        break;
                    case 'weight':
                        data.weight_kg = value as number;
                        break;
                    case 'measurements':
                        data.measurements = value as string;
                        break;
                    case 'cupsize':
                        data.cupsize = value as string;
                        break;
                    case 'hair_colour':
                        data.hair_color = value as string;
                        break;
                    case 'eye_colour':
                        data.eye_color = value as string;
                        break;
                    case 'tattoos':
                        data.tattoos = value as string;
                        break;
                    case 'piercings':
                        data.piercings = value as string;
                        break;
                    case 'fake_boobs':
                        data.fake_boobs = value as boolean;
                        break;
                    case 'same_sex_only':
                        data.same_sex_only = value as boolean;
                        break;
                }
            }
        }
    }

    emit('apply', data);
};

// Select all fields with changes
const selectAllChanged = () => {
    for (const field of fieldDefinitions) {
        if (hasFieldChanged(field)) {
            selectedFields.value[field.key] = true;
        }
    }
};

// Deselect all fields
const deselectAll = () => {
    for (const field of fieldDefinitions) {
        selectedFields.value[field.key] = false;
    }
};

// Handle going back - reset selected fields
const handleGoBack = () => {
    selectedFields.value = {};
    goBack();
};

// Handle close
const handleClose = () => {
    emit('close');
};

// Initialize field selection when performer details are loaded
watch(performerDetails, (details) => {
    if (details) {
        initializeFieldSelection();
    }
});

// Initialize search when modal opens
watch(
    () => props.visible,
    (visible) => {
        if (visible && props.actorName) {
            searchQuery.value = props.actorName;
            searchPerformers(props.actorName);
        }
    },
    { immediate: true },
);

// Cleanup on unmount
onUnmounted(cleanup);
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70
                p-4 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border my-8 w-full max-w-3xl border p-6">
                <!-- Header -->
                <div class="mb-4 flex items-center justify-between">
                    <div class="flex items-center gap-2">
                        <button
                            v-if="selectedPerformer"
                            class="text-dim transition-colors hover:text-white"
                            @click="handleGoBack"
                        >
                            <Icon name="heroicons:arrow-left" size="18" />
                        </button>
                        <h3 class="text-sm font-semibold text-white">
                            {{ selectedPerformer ? 'Preview Metadata' : 'Fetch Actor Metadata' }}
                        </h3>
                    </div>
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="handleClose"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Search Section (when no performer selected) -->
                <div v-if="!selectedPerformer">
                    <!-- Search Input -->
                    <div class="mb-4 flex gap-2">
                        <input
                            v-model="searchQuery"
                            type="text"
                            placeholder="Search performer name..."
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 flex-1 rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                            @keyup.enter="searchPerformers(searchQuery)"
                        />
                        <button
                            :disabled="isSearching || !searchQuery.trim()"
                            class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg
                                px-4 py-2 text-xs font-semibold text-white transition-all
                                disabled:cursor-not-allowed disabled:opacity-40"
                            @click="searchPerformers(searchQuery)"
                        >
                            <Icon
                                v-if="isSearching"
                                name="heroicons:arrow-path"
                                size="14"
                                class="animate-spin"
                            />
                            <Icon v-else name="heroicons:magnifying-glass" size="14" />
                            Search
                        </button>
                    </div>

                    <!-- Search Error -->
                    <div
                        v-if="searchError"
                        class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2
                            text-xs"
                    >
                        {{ searchError }}
                    </div>

                    <!-- Search Results -->
                    <div
                        v-if="searchResults.length > 0"
                        class="border-border max-h-96 space-y-1 overflow-y-auto rounded-lg border
                            p-2"
                    >
                        <button
                            v-for="performer in searchResults"
                            :key="performer.id"
                            class="border-border hover:border-lava/30 hover:bg-lava/5 flex w-full
                                items-center gap-3 rounded-lg border p-2 text-left
                                transition-colors"
                            :class="{
                                'pointer-events-none opacity-50':
                                    isFetchingDetails && selectedPerformerId !== performer.id,
                            }"
                            @click="handleFetchPerformerDetails(performer)"
                            @mouseenter="handleHover(performer)"
                            @mouseleave="handleHoverLeave"
                        >
                            <div
                                class="bg-surface border-border h-12 w-9 shrink-0 overflow-hidden
                                    rounded border"
                            >
                                <img
                                    v-if="performer.image"
                                    :src="performer.image"
                                    :alt="performer.name"
                                    class="h-full w-full object-cover"
                                    loading="lazy"
                                />
                                <div
                                    v-else
                                    class="text-dim flex h-full w-full items-center justify-center"
                                >
                                    <Icon name="heroicons:user" size="16" />
                                </div>
                            </div>
                            <div class="min-w-0 flex-1">
                                <div class="truncate text-sm font-medium text-white">
                                    {{ performer.name }}
                                </div>
                                <div v-if="performer.bio" class="text-dim mt-0.5 truncate text-xs">
                                    {{ performer.bio }}
                                </div>
                            </div>
                            <!-- Prefetch indicator or chevron -->
                            <Icon
                                v-if="prefetchingId === performer.id"
                                name="heroicons:arrow-path"
                                size="14"
                                class="text-dim shrink-0 animate-spin"
                            />
                            <Icon
                                v-else
                                name="heroicons:chevron-right"
                                size="16"
                                class="text-dim shrink-0"
                            />
                        </button>
                    </div>

                    <!-- Empty State -->
                    <div
                        v-else-if="!isSearching && !searchError"
                        class="text-dim py-8 text-center text-sm"
                    >
                        Search ThePornDB to find actor metadata
                    </div>

                    <!-- Loading State -->
                    <div v-if="isSearching" class="flex items-center justify-center py-8">
                        <LoadingSpinner label="Searching..." />
                    </div>
                </div>

                <!-- Preview Section (when performer selected) -->
                <div v-else>
                    <!-- Loading Details -->
                    <div v-if="isFetchingDetails" class="flex items-center justify-center py-12">
                        <LoadingSpinner label="Loading performer details..." />
                    </div>

                    <!-- Details Error -->
                    <div
                        v-else-if="detailsError"
                        class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2
                            text-xs"
                    >
                        {{ detailsError }}
                    </div>

                    <!-- Preview Content -->
                    <div v-else-if="performerDetails">
                        <!-- Image Comparison -->
                        <div class="mb-4 grid grid-cols-2 gap-4">
                            <!-- Current Image -->
                            <div>
                                <div
                                    class="text-dim mb-1.5 text-[11px] font-medium tracking-wider
                                        uppercase"
                                >
                                    Current
                                </div>
                                <div
                                    class="bg-surface border-border mx-auto h-32 w-24
                                        overflow-hidden rounded-lg border"
                                >
                                    <img
                                        v-if="currentActor.image_url"
                                        :src="currentActor.image_url"
                                        :alt="currentActor.name"
                                        class="h-full w-full object-cover"
                                    />
                                    <div
                                        v-else
                                        class="text-dim flex h-full w-full items-center
                                            justify-center"
                                    >
                                        <Icon name="heroicons:user" size="32" />
                                    </div>
                                </div>
                            </div>
                            <!-- PornDB Image -->
                            <div>
                                <div
                                    class="text-dim mb-1.5 text-[11px] font-medium tracking-wider
                                        uppercase"
                                >
                                    ThePornDB
                                </div>
                                <div
                                    class="bg-surface border-border mx-auto h-32 w-24
                                        overflow-hidden rounded-lg border"
                                >
                                    <img
                                        v-if="performerDetails.image"
                                        :src="performerDetails.image"
                                        :alt="performerDetails.name"
                                        class="h-full w-full object-cover"
                                    />
                                    <div
                                        v-else
                                        class="text-dim flex h-full w-full items-center
                                            justify-center"
                                    >
                                        <Icon name="heroicons:user" size="32" />
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Selection Controls -->
                        <div class="mb-3 flex items-center justify-between">
                            <span class="text-dim text-xs">
                                {{ selectedFieldCount }} field{{
                                    selectedFieldCount === 1 ? '' : 's'
                                }}
                                selected
                            </span>
                            <div class="flex gap-2">
                                <button
                                    class="text-lava hover:text-lava-glow text-xs transition-colors"
                                    @click="selectAllChanged"
                                >
                                    Select all with changes
                                </button>
                                <span class="text-dim">|</span>
                                <button
                                    class="text-dim text-xs transition-colors hover:text-white"
                                    @click="deselectAll"
                                >
                                    Deselect all
                                </button>
                            </div>
                        </div>

                        <!-- Fields Comparison -->
                        <div
                            class="border-border max-h-72 space-y-1 overflow-y-auto rounded-lg
                                border p-2"
                        >
                            <div
                                v-for="field in fieldDefinitions"
                                :key="field.key"
                                class="flex items-center gap-3 rounded px-2 py-1.5 text-sm
                                    hover:bg-white/[0.02]"
                                :class="{
                                    'bg-lava/5': selectedFields[field.key],
                                }"
                            >
                                <!-- Checkbox -->
                                <input
                                    v-model="selectedFields[field.key]"
                                    type="checkbox"
                                    class="accent-lava h-3.5 w-3.5 shrink-0 rounded"
                                    :disabled="!hasFieldChanged(field)"
                                />

                                <!-- Field Name -->
                                <div class="w-28 shrink-0">
                                    <span
                                        class="text-xs"
                                        :class="[
                                            hasFieldChanged(field) ? 'text-white' : 'text-dim',
                                        ]"
                                    >
                                        {{ field.label }}
                                    </span>
                                </div>

                                <!-- Current Value -->
                                <div class="min-w-0 flex-1">
                                    <span class="text-dim truncate text-xs">
                                        {{
                                            formatValue(
                                                getCurrentValue(field.actorKey),
                                                field.format,
                                            )
                                        }}
                                    </span>
                                </div>

                                <!-- Arrow -->
                                <Icon
                                    name="heroicons:arrow-right"
                                    size="12"
                                    class="shrink-0"
                                    :class="[hasFieldChanged(field) ? 'text-lava' : 'text-dim/30']"
                                />

                                <!-- PornDB Value -->
                                <div class="min-w-0 flex-1">
                                    <span
                                        class="truncate text-xs"
                                        :class="[
                                            hasFieldChanged(field) ? 'text-white' : 'text-dim',
                                        ]"
                                    >
                                        {{ formatValue(getPornDBValue(field.key), field.format) }}
                                    </span>
                                </div>
                            </div>
                        </div>

                        <!-- Actions -->
                        <div class="mt-4 flex justify-end gap-2">
                            <button
                                class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                    hover:text-white"
                                @click="handleClose"
                            >
                                Cancel
                            </button>
                            <button
                                :disabled="selectedFieldCount === 0"
                                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                    font-semibold text-white transition-all
                                    disabled:cursor-not-allowed disabled:opacity-40"
                                @click="applyMetadata"
                            >
                                Apply Selected
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </Teleport>
</template>
