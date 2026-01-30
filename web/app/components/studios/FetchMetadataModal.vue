<script setup lang="ts">
import type { Studio, UpdateStudioInput } from '~/types/studio';
import type { PornDBSiteDetails } from '~/types/porndb';

interface PornDBSiteSearchResult {
    id: string;
    name: string;
    short_name?: string;
    logo?: string;
}

const props = defineProps<{
    visible: boolean;
    studioName: string;
    currentStudio: Studio;
}>();

const emit = defineEmits<{
    close: [];
    apply: [data: Partial<UpdateStudioInput>];
}>();

const api = useApi();

// State
const searchQuery = ref(props.studioName);
const isSearching = ref(false);
const searchResults = ref<PornDBSiteSearchResult[]>([]);
const searchError = ref<string | null>(null);

const selectedSite = ref<PornDBSiteSearchResult | null>(null);
const isFetchingDetails = ref(false);
const siteDetails = ref<PornDBSiteDetails | null>(null);
const detailsError = ref<string | null>(null);

// Field selection for applying metadata
const selectedFields = ref<Record<string, boolean>>({});

// Field definitions for display
const fieldDefinitions: {
    key: keyof PornDBSiteDetails;
    studioKey: keyof Studio;
    label: string;
    format?: (val: unknown) => string;
}[] = [
    { key: 'name', studioKey: 'name', label: 'Name' },
    { key: 'short_name', studioKey: 'short_name', label: 'Short Name' },
    { key: 'logo', studioKey: 'logo', label: 'Logo' },
    { key: 'url', studioKey: 'url', label: 'Website' },
    { key: 'description', studioKey: 'description', label: 'Description' },
    {
        key: 'rating',
        studioKey: 'rating',
        label: 'Rating',
        format: (val) => (val ? String(val) : '-'),
    },
];

// Search sites
const searchSites = async () => {
    if (!searchQuery.value.trim()) return;

    isSearching.value = true;
    searchError.value = null;
    searchResults.value = [];
    selectedSite.value = null;
    siteDetails.value = null;

    try {
        const results = await api.searchPornDBSites(searchQuery.value);
        searchResults.value = results;

        if (searchResults.value.length === 0) {
            searchError.value = 'No sites found';
        }
    } catch (err) {
        searchError.value = err instanceof Error ? err.message : 'Search failed';
    } finally {
        isSearching.value = false;
    }
};

// Fetch site details
const fetchSiteDetails = async (site: PornDBSiteSearchResult) => {
    selectedSite.value = site;
    isFetchingDetails.value = true;
    detailsError.value = null;
    siteDetails.value = null;
    selectedFields.value = {};

    try {
        const details = await api.getPornDBSite(site.id);
        siteDetails.value = details;

        // Pre-select fields that have new data and current studio is missing
        initializeFieldSelection();
    } catch (err) {
        detailsError.value = err instanceof Error ? err.message : 'Failed to fetch details';
    } finally {
        isFetchingDetails.value = false;
    }
};

// Initialize field selection - pre-check fields where we have new data and current is empty
const initializeFieldSelection = () => {
    if (!siteDetails.value) return;

    for (const field of fieldDefinitions) {
        const porndbValue = siteDetails.value[field.key];
        const currentValue = props.currentStudio[field.studioKey];

        // Pre-select if PornDB has data and current value is empty/null/undefined
        const hasNewData = porndbValue !== null && porndbValue !== undefined && porndbValue !== '';
        const currentIsEmpty =
            currentValue === null || currentValue === undefined || currentValue === '';

        selectedFields.value[field.key] = hasNewData && currentIsEmpty;
    }
};

// Format value for display
const formatValue = (value: unknown, format?: (val: unknown) => string): string => {
    if (value === null || value === undefined || value === '') return '-';
    if (format) return format(value);
    return String(value);
};

// Get current studio value for a field
const getCurrentValue = (studioKey: keyof Studio): unknown => {
    return props.currentStudio[studioKey];
};

// Get PornDB value for a field
const getPornDBValue = (key: keyof PornDBSiteDetails): unknown => {
    return siteDetails.value?.[key];
};

// Check if a field has changed
const hasFieldChanged = (field: (typeof fieldDefinitions)[0]): boolean => {
    const porndbValue = getPornDBValue(field.key);
    const currentValue = getCurrentValue(field.studioKey);

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
    if (!siteDetails.value) return;

    const data: Partial<UpdateStudioInput> = {};

    for (const field of fieldDefinitions) {
        if (selectedFields.value[field.key]) {
            const value = siteDetails.value[field.key];
            if (value !== null && value !== undefined && value !== '') {
                // Map PornDB field to studio field
                switch (field.key) {
                    case 'name':
                        data.name = value as string;
                        break;
                    case 'short_name':
                        data.short_name = value as string;
                        break;
                    case 'logo':
                        data.logo = value as string;
                        break;
                    case 'url':
                        data.url = value as string;
                        break;
                    case 'description':
                        data.description = value as string;
                        break;
                    case 'rating':
                        data.rating = value as number;
                        break;
                }
            }
        }
    }

    // Also store the PornDB ID
    if (siteDetails.value.id) {
        data.porndb_id = siteDetails.value.id;
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

// Go back to search results
const goBack = () => {
    selectedSite.value = null;
    siteDetails.value = null;
    detailsError.value = null;
    selectedFields.value = {};
};

// Handle close
const handleClose = () => {
    emit('close');
};

// Search on mount
onMounted(() => {
    if (searchQuery.value) {
        searchSites();
    }
});
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
                            v-if="selectedSite"
                            @click="goBack"
                            class="text-dim transition-colors hover:text-white"
                        >
                            <Icon name="heroicons:arrow-left" size="18" />
                        </button>
                        <h3 class="text-sm font-semibold text-white">
                            {{ selectedSite ? 'Preview Metadata' : 'Fetch Studio Metadata' }}
                        </h3>
                    </div>
                    <button
                        @click="handleClose"
                        class="text-dim transition-colors hover:text-white"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Search Section (when no site selected) -->
                <div v-if="!selectedSite">
                    <!-- Search Input -->
                    <div class="mb-4 flex gap-2">
                        <input
                            v-model="searchQuery"
                            type="text"
                            placeholder="Search studio/site name..."
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 flex-1 rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                            @keyup.enter="searchSites"
                        />
                        <button
                            @click="searchSites"
                            :disabled="isSearching || !searchQuery.trim()"
                            class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg
                                px-4 py-2 text-xs font-semibold text-white transition-all
                                disabled:cursor-not-allowed disabled:opacity-40"
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
                            v-for="site in searchResults"
                            :key="site.id"
                            @click="fetchSiteDetails(site)"
                            class="border-border hover:border-lava/30 hover:bg-lava/5 flex w-full
                                items-center gap-3 rounded-lg border p-2 text-left
                                transition-colors"
                        >
                            <div
                                class="bg-surface border-border h-12 w-12 shrink-0 overflow-hidden
                                    rounded border"
                            >
                                <img
                                    v-if="site.logo"
                                    :src="site.logo"
                                    :alt="site.name"
                                    class="h-full w-full object-contain p-1"
                                    loading="lazy"
                                />
                                <div
                                    v-else
                                    class="text-dim flex h-full w-full items-center justify-center"
                                >
                                    <Icon name="heroicons:building-office-2" size="16" />
                                </div>
                            </div>
                            <div class="min-w-0 flex-1">
                                <div class="truncate text-sm font-medium text-white">
                                    {{ site.name }}
                                </div>
                                <div
                                    v-if="site.short_name"
                                    class="text-dim mt-0.5 truncate text-xs"
                                >
                                    {{ site.short_name }}
                                </div>
                            </div>
                            <Icon
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
                        Search ThePornDB to find studio metadata
                    </div>

                    <!-- Loading State -->
                    <div v-if="isSearching" class="flex items-center justify-center py-8">
                        <LoadingSpinner label="Searching..." />
                    </div>
                </div>

                <!-- Preview Section (when site selected) -->
                <div v-else>
                    <!-- Loading Details -->
                    <div v-if="isFetchingDetails" class="flex items-center justify-center py-12">
                        <LoadingSpinner label="Loading site details..." />
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
                    <div v-else-if="siteDetails">
                        <!-- Logo Comparison -->
                        <div class="mb-4 grid grid-cols-2 gap-4">
                            <!-- Current Logo -->
                            <div>
                                <div
                                    class="text-dim mb-1.5 text-[11px] font-medium tracking-wider
                                        uppercase"
                                >
                                    Current
                                </div>
                                <div
                                    class="bg-surface border-border mx-auto h-24 w-24
                                        overflow-hidden rounded-lg border"
                                >
                                    <img
                                        v-if="currentStudio.logo"
                                        :src="currentStudio.logo"
                                        :alt="currentStudio.name"
                                        class="h-full w-full object-contain p-2"
                                    />
                                    <div
                                        v-else
                                        class="text-dim flex h-full w-full items-center
                                            justify-center"
                                    >
                                        <Icon name="heroicons:building-office-2" size="32" />
                                    </div>
                                </div>
                            </div>
                            <!-- PornDB Logo -->
                            <div>
                                <div
                                    class="text-dim mb-1.5 text-[11px] font-medium tracking-wider
                                        uppercase"
                                >
                                    ThePornDB
                                </div>
                                <div
                                    class="bg-surface border-border mx-auto h-24 w-24
                                        overflow-hidden rounded-lg border"
                                >
                                    <img
                                        v-if="siteDetails.logo"
                                        :src="siteDetails.logo"
                                        :alt="siteDetails.name"
                                        class="h-full w-full object-contain p-2"
                                    />
                                    <div
                                        v-else
                                        class="text-dim flex h-full w-full items-center
                                            justify-center"
                                    >
                                        <Icon name="heroicons:building-office-2" size="32" />
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
                                    @click="selectAllChanged"
                                    class="text-lava hover:text-lava-glow text-xs transition-colors"
                                >
                                    Select all with changes
                                </button>
                                <span class="text-dim">|</span>
                                <button
                                    @click="deselectAll"
                                    class="text-dim text-xs transition-colors hover:text-white"
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
                                                getCurrentValue(field.studioKey),
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
                                @click="handleClose"
                                class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                    hover:text-white"
                            >
                                Cancel
                            </button>
                            <button
                                @click="applyMetadata"
                                :disabled="selectedFieldCount === 0"
                                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                    font-semibold text-white transition-all
                                    disabled:cursor-not-allowed disabled:opacity-40"
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
