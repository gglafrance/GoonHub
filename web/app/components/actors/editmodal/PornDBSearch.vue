<script setup lang="ts">
import type { PornDBPerformer, PornDBPerformerDetails } from '~/types/porndb';

type FormState = {
    name: string;
    aliases: string[];
    image_url: string;
    gender: string;
    birthday: string;
    date_of_death: string;
    astrology: string;
    birthplace: string;
    ethnicity: string;
    nationality: string;
    career_start_year: number | null;
    career_end_year: number | null;
    height_cm: number | null;
    weight_kg: number | null;
    measurements: string;
    cupsize: string;
    hair_color: string;
    eye_color: string;
    tattoos: string;
    piercings: string;
    fake_boobs: boolean;
    same_sex_only: boolean;
};

const props = defineProps<{
    actorName: string;
    form: FormState;
}>();

const emit = defineEmits<{
    apply: [data: Record<string, unknown>];
    back: [];
}>();

const apiPornDB = useApiPornDB();

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

const selectedPerformerId = computed(() => selectedPerformer.value?.id ?? null);

const selectedFields = ref<Record<string, boolean>>({});

// Field definitions mapping PornDB keys to form keys
const fieldDefinitions: {
    key: keyof PornDBPerformerDetails;
    formKey: keyof FormState;
    label: string;
    format?: (val: unknown) => string;
}[] = [
    { key: 'name', formKey: 'name', label: 'Name' },
    {
        key: 'aliases',
        formKey: 'aliases',
        label: 'Aliases',
        format: (val) => (val as string[])?.join(', ') || '-',
    },
    { key: 'image', formKey: 'image_url', label: 'Image' },
    { key: 'gender', formKey: 'gender', label: 'Gender' },
    {
        key: 'birthday',
        formKey: 'birthday',
        label: 'Birthday',
        format: (val) => (val ? new Date(val as string).toLocaleDateString() : '-'),
    },
    {
        key: 'deathday',
        formKey: 'date_of_death',
        label: 'Date of Death',
        format: (val) => (val ? new Date(val as string).toLocaleDateString() : '-'),
    },
    { key: 'astrology', formKey: 'astrology', label: 'Astrology' },
    { key: 'birthplace', formKey: 'birthplace', label: 'Birthplace' },
    { key: 'ethnicity', formKey: 'ethnicity', label: 'Ethnicity' },
    { key: 'nationality', formKey: 'nationality', label: 'Nationality' },
    { key: 'career_start_year', formKey: 'career_start_year', label: 'Career Start' },
    { key: 'career_end_year', formKey: 'career_end_year', label: 'Career End' },
    {
        key: 'height',
        formKey: 'height_cm',
        label: 'Height',
        format: (val) => (val ? `${val}cm` : '-'),
    },
    {
        key: 'weight',
        formKey: 'weight_kg',
        label: 'Weight',
        format: (val) => (val ? `${val}kg` : '-'),
    },
    { key: 'measurements', formKey: 'measurements', label: 'Measurements' },
    { key: 'cupsize', formKey: 'cupsize', label: 'Cup Size' },
    { key: 'hair_colour', formKey: 'hair_color', label: 'Hair Color' },
    { key: 'eye_colour', formKey: 'eye_color', label: 'Eye Color' },
    { key: 'tattoos', formKey: 'tattoos', label: 'Tattoos' },
    { key: 'piercings', formKey: 'piercings', label: 'Piercings' },
    {
        key: 'fake_boobs',
        formKey: 'fake_boobs',
        label: 'Enhanced',
        format: (val) => (val ? 'Yes' : 'No'),
    },
    {
        key: 'same_sex_only',
        formKey: 'same_sex_only',
        label: 'Same-sex Only',
        format: (val) => (val ? 'Yes' : 'No'),
    },
];

const handleFetchPerformerDetails = (performer: PornDBPerformer) => {
    selectedFields.value = {};
    fetchPerformerDetails(performer);
};

const initializeFieldSelection = () => {
    if (!performerDetails.value) return;

    for (const field of fieldDefinitions) {
        const porndbValue = performerDetails.value[field.key];
        const currentValue = props.form[field.formKey];

        const hasNewData = Array.isArray(porndbValue)
            ? porndbValue.length > 0
            : porndbValue !== null && porndbValue !== undefined && porndbValue !== '';
        const currentIsEmpty = Array.isArray(currentValue)
            ? currentValue.length === 0
            : currentValue === null || currentValue === undefined || currentValue === '';

        selectedFields.value[field.key] = hasNewData && currentIsEmpty;
    }
};

const formatValue = (value: unknown, format?: (val: unknown) => string): string => {
    if (value === null || value === undefined || value === '') return '-';
    if (format) return format(value);
    return String(value);
};

const getCurrentValue = (formKey: keyof FormState): unknown => {
    return props.form[formKey];
};

const getPornDBValue = (key: keyof PornDBPerformerDetails): unknown => {
    return performerDetails.value?.[key];
};

const hasFieldChanged = (field: (typeof fieldDefinitions)[0]): boolean => {
    const porndbValue = getPornDBValue(field.key);
    const currentValue = getCurrentValue(field.formKey);

    if (Array.isArray(porndbValue) || Array.isArray(currentValue)) {
        const pArr = (porndbValue as string[]) || [];
        const cArr = (currentValue as string[]) || [];
        if (pArr.length === 0) return false;
        if (pArr.length !== cArr.length) return true;
        return pArr.some((v, i) => v !== cArr[i]);
    }

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

const selectedFieldCount = computed(() => {
    return Object.values(selectedFields.value).filter(Boolean).length;
});

const applyToForm = () => {
    if (!performerDetails.value) return;

    const data: Record<string, unknown> = {};

    for (const field of fieldDefinitions) {
        if (selectedFields.value[field.key]) {
            const value = performerDetails.value[field.key];
            if (value !== null && value !== undefined && value !== '') {
                // Map PornDB key to form key
                switch (field.key) {
                    case 'image':
                        data.image_url = value;
                        break;
                    case 'deathday':
                        data.date_of_death = value;
                        break;
                    case 'height':
                        data.height_cm = value;
                        break;
                    case 'weight':
                        data.weight_kg = value;
                        break;
                    case 'hair_colour':
                        data.hair_color = value;
                        break;
                    case 'eye_colour':
                        data.eye_color = value;
                        break;
                    default:
                        // Keys that match directly between PornDB and form
                        data[field.formKey] = value;
                        break;
                }
            }
        }
    }

    emit('apply', data);
};

const selectAllChanged = () => {
    for (const field of fieldDefinitions) {
        if (hasFieldChanged(field)) {
            selectedFields.value[field.key] = true;
        }
    }
};

const deselectAll = () => {
    for (const field of fieldDefinitions) {
        selectedFields.value[field.key] = false;
    }
};

const handleGoBack = () => {
    selectedFields.value = {};
    goBack();
};

const handleBack = () => {
    emit('back');
};

watch(performerDetails, (details) => {
    if (details) {
        initializeFieldSelection();
    }
});

// Auto-search on mount
onMounted(() => {
    if (props.actorName) {
        searchQuery.value = props.actorName;
        searchPerformers(props.actorName);
    }
});

onUnmounted(cleanup);
</script>

<template>
    <div>
        <!-- Header -->
        <div class="mb-4 flex items-center justify-between">
            <div class="flex items-center gap-2">
                <button
                    class="text-dim transition-colors hover:text-white"
                    @click="selectedPerformer ? handleGoBack() : handleBack()"
                >
                    <Icon name="heroicons:arrow-left" size="18" />
                </button>
                <h3 class="text-sm font-semibold text-white">
                    {{ selectedPerformer ? 'Preview Metadata' : 'Search ThePornDB' }}
                </h3>
            </div>
        </div>

        <!-- Search Section -->
        <div v-if="!selectedPerformer">
            <div class="mb-4 flex gap-2">
                <input
                    v-model="searchQuery"
                    type="text"
                    placeholder="Search performer name..."
                    class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                        focus:ring-lava/20 flex-1 rounded-lg border px-3 py-2 text-sm text-white
                        transition-all focus:ring-1 focus:outline-none"
                    @keyup.enter="searchPerformers(searchQuery)"
                />
                <button
                    :disabled="isSearching || !searchQuery.trim()"
                    class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-4 py-2
                        text-xs font-semibold text-white transition-all disabled:cursor-not-allowed
                        disabled:opacity-40"
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

            <div
                v-if="searchError"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ searchError }}
            </div>

            <div
                v-if="searchResults.length > 0"
                class="border-border max-h-96 space-y-1 overflow-y-auto rounded-lg border p-2"
            >
                <button
                    v-for="performer in searchResults"
                    :key="performer.id"
                    class="border-border hover:border-lava/30 hover:bg-lava/5 flex w-full
                        items-center gap-3 rounded-lg border p-2 text-left transition-colors"
                    :class="{
                        'pointer-events-none opacity-50':
                            isFetchingDetails && selectedPerformerId !== performer.id,
                    }"
                    @click="handleFetchPerformerDetails(performer)"
                    @mouseenter="handleHover(performer)"
                    @mouseleave="handleHoverLeave"
                >
                    <div
                        class="bg-surface border-border h-12 w-9 shrink-0 overflow-hidden rounded
                            border"
                    >
                        <img
                            v-if="performer.image"
                            :src="performer.image"
                            :alt="performer.name"
                            class="h-full w-full object-cover"
                            loading="lazy"
                        />
                        <div v-else class="text-dim flex h-full w-full items-center justify-center">
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

            <div v-else-if="!isSearching && !searchError" class="text-dim py-8 text-center text-sm">
                Search ThePornDB to find actor metadata
            </div>

            <div v-if="isSearching" class="flex items-center justify-center py-8">
                <LoadingSpinner label="Searching..." />
            </div>
        </div>

        <!-- Preview Section -->
        <div v-else>
            <div v-if="isFetchingDetails" class="flex items-center justify-center py-12">
                <LoadingSpinner label="Loading performer details..." />
            </div>

            <div
                v-else-if="detailsError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ detailsError }}
            </div>

            <div v-else-if="performerDetails">
                <!-- Image Comparison -->
                <div class="mb-4 grid grid-cols-2 gap-4">
                    <div>
                        <div
                            class="text-dim mb-1.5 text-[11px] font-medium tracking-wider uppercase"
                        >
                            Current
                        </div>
                        <div
                            class="bg-surface border-border mx-auto h-32 w-24 overflow-hidden
                                rounded-lg border"
                        >
                            <img
                                v-if="form.image_url"
                                :src="form.image_url"
                                :alt="form.name"
                                class="h-full w-full object-cover"
                            />
                            <div
                                v-else
                                class="text-dim flex h-full w-full items-center justify-center"
                            >
                                <Icon name="heroicons:user" size="32" />
                            </div>
                        </div>
                    </div>
                    <div>
                        <div
                            class="text-dim mb-1.5 text-[11px] font-medium tracking-wider uppercase"
                        >
                            ThePornDB
                        </div>
                        <div
                            class="bg-surface border-border mx-auto h-32 w-24 overflow-hidden
                                rounded-lg border"
                        >
                            <img
                                v-if="performerDetails.image"
                                :src="performerDetails.image"
                                :alt="performerDetails.name"
                                class="h-full w-full object-cover"
                            />
                            <div
                                v-else
                                class="text-dim flex h-full w-full items-center justify-center"
                            >
                                <Icon name="heroicons:user" size="32" />
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Selection Controls -->
                <div class="mb-3 flex items-center justify-between">
                    <span class="text-dim text-xs">
                        {{ selectedFieldCount }} field{{ selectedFieldCount === 1 ? '' : 's' }}
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
                <div class="border-border max-h-72 space-y-1 overflow-y-auto rounded-lg border p-2">
                    <div
                        v-for="field in fieldDefinitions"
                        :key="field.key"
                        class="flex items-center gap-3 rounded px-2 py-1.5 text-sm
                            hover:bg-white/[0.02]"
                        :class="{
                            'bg-lava/5': selectedFields[field.key],
                        }"
                    >
                        <input
                            v-model="selectedFields[field.key]"
                            type="checkbox"
                            class="accent-lava h-3.5 w-3.5 shrink-0 rounded"
                            :disabled="!hasFieldChanged(field)"
                        />
                        <div class="w-28 shrink-0">
                            <span
                                class="text-xs"
                                :class="[hasFieldChanged(field) ? 'text-white' : 'text-dim']"
                            >
                                {{ field.label }}
                            </span>
                        </div>
                        <div class="min-w-0 flex-1">
                            <span class="text-dim truncate text-xs">
                                {{ formatValue(getCurrentValue(field.formKey), field.format) }}
                            </span>
                        </div>
                        <Icon
                            name="heroicons:arrow-right"
                            size="12"
                            class="shrink-0"
                            :class="[hasFieldChanged(field) ? 'text-lava' : 'text-dim/30']"
                        />
                        <div class="min-w-0 flex-1">
                            <span
                                class="truncate text-xs"
                                :class="[hasFieldChanged(field) ? 'text-white' : 'text-dim']"
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
                        @click="handleBack"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="selectedFieldCount === 0"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                            font-semibold text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="applyToForm"
                    >
                        Apply to Form
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
