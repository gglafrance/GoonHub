<script setup lang="ts">
import type { SearchStatusKey } from '~/stores/search';
import { SEARCH_STATUS_KEYS } from '~/stores/search';
const searchStore = useSearchStore();
// Size filters are not applied without files:view_size (see the search store)
const perms = useMyPermissions();

const removeTag = (tag: string) => {
    searchStore.selectedTags = searchStore.selectedTags.filter((t) => t !== tag);
};

const removeActor = (actor: string) => {
    searchStore.selectedActors = searchStore.selectedActors.filter((a) => a !== actor);
};

const removeSourceSite = (site: string) => {
    searchStore.selectedSourceSites = searchStore.selectedSourceSites.filter((s) => s !== site);
};

const removeMarkerLabel = (label: string) => {
    searchStore.selectedMarkerLabels = searchStore.selectedMarkerLabels.filter((l) => l !== label);
};

// Size chip label (bounds in megabytes)
const formatMB = (mb: number) => (mb >= 1024 ? `${+(mb / 1024).toFixed(1)} GB` : `${mb} MB`);
const formatSizeRange = (min: number, max: number) => {
    if (min > 0 && max > 0) return `${formatMB(min)} - ${formatMB(max)}`;
    if (min > 0) return `${formatMB(min)}+`;
    return `< ${formatMB(max)}`;
};

const STATUS_LABELS: Record<SearchStatusKey, [string, string]> = {
    watched: ['Watched', 'Never watched'],
    rated: ['Rated', 'Not rated'],
    has_markers: ['With markers', 'Without markers'],
    has_actors: ['With actors', 'Without actors'],
    has_tags: ['With tags', 'Without tags'],
    has_studio: ['With studio', 'Without studio'],
    has_source: ['With source', 'Without source'],
};

const statusChips = computed(() =>
    SEARCH_STATUS_KEYS.filter((k) => searchStore.status[k] !== null).map((k) => ({
        key: k,
        label: STATUS_LABELS[k][searchStore.status[k] ? 0 : 1],
    })),
);
</script>

<template>
    <div class="flex flex-wrap gap-1.5">
        <div v-if="!searchStore.hasActiveFilters">
            <span class="text-dim text-sm">No active filters</span>
        </div>

        <span
            v-if="searchStore.query"
            class="bg-lava/10 text-lava inline-flex items-center gap-1 rounded-full px-2.5 py-0.5
                text-[11px] font-medium"
        >
            "{{ searchStore.query }}"
            <button class="hover:text-white" @click="searchStore.query = ''">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-for="tag in searchStore.selectedTags"
            :key="'tag-' + tag"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            {{ tag }}
            <button class="text-dim hover:text-white" @click="removeTag(tag)">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-for="actor in searchStore.selectedActors"
            :key="'actor-' + actor"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            {{ actor }}
            <button class="text-dim hover:text-white" @click="removeActor(actor)">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-for="label in searchStore.selectedMarkerLabels"
            :key="'marker-' + label"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            <Icon name="heroicons:bookmark" size="10" class="opacity-50" />
            {{ label }}
            <button class="text-dim hover:text-white" @click="removeMarkerLabel(label)">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.studio"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            Studio: {{ searchStore.studio }}
            <button class="text-dim hover:text-white" @click="searchStore.studio = ''">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-for="site in searchStore.selectedSourceSites"
            :key="'source-' + site"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            <Icon name="heroicons:globe-alt" size="12" class="opacity-50" />
            {{ site }}
            <button class="text-dim hover:text-white" @click="removeSourceSite(site)">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.minDuration > 0 || searchStore.maxDuration > 0"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            <template v-if="searchStore.minDuration > 0 && searchStore.maxDuration > 0">
                {{ Math.floor(searchStore.minDuration / 60) }}-{{
                    Math.floor(searchStore.maxDuration / 60)
                }}
                min
            </template>
            <template v-else-if="searchStore.minDuration > 0">
                {{ Math.floor(searchStore.minDuration / 60) }}+ min
            </template>
            <template v-else> &lt; {{ Math.floor(searchStore.maxDuration / 60) }} min </template>
            <button
                class="text-dim hover:text-white"
                @click="
                    searchStore.minDuration = 0;
                    searchStore.maxDuration = 0;
                "
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>
        <span
            v-for="chip in statusChips"
            :key="chip.key"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            <Icon name="heroicons:check-badge" size="12" />
            {{ chip.label }}
            <button
                class="hover:text-white"
                @click="
                    searchStore.status[chip.key] = null;
                    searchStore.search();
                "
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>
        <span
            v-if="
                perms.has('files:view_size') && (searchStore.minSize > 0 || searchStore.maxSize > 0)
            "
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            {{ formatSizeRange(searchStore.minSize, searchStore.maxSize) }}
            <button
                class="text-dim hover:text-white"
                @click="
                    searchStore.minSize = 0;
                    searchStore.maxSize = 0;
                "
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.resolution"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            {{ searchStore.resolution }}
            <button class="text-dim hover:text-white" @click="searchStore.resolution = ''">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.minDate || searchStore.maxDate"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            {{ searchStore.minDate || 'start' }} - {{ searchStore.maxDate || 'now' }}
            <button
                class="text-dim hover:text-white"
                @click="
                    searchStore.minDate = '';
                    searchStore.maxDate = '';
                "
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.liked"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            Liked
            <button class="text-dim hover:text-white" @click="searchStore.liked = false">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.minRating > 0 || searchStore.maxRating > 0"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            Rating:
            <template v-if="searchStore.minRating > 0 && searchStore.maxRating > 0">
                {{ searchStore.minRating }}-{{ searchStore.maxRating }}
            </template>
            <template v-else-if="searchStore.minRating > 0">
                {{ searchStore.minRating }}+
            </template>
            <template v-else> &lt;{{ searchStore.maxRating }} </template>
            <button
                class="text-dim hover:text-white"
                @click="
                    searchStore.minRating = 0;
                    searchStore.maxRating = 0;
                "
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.minJizzCount > 0 || searchStore.maxJizzCount > 0"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            Jizz:
            <template v-if="searchStore.minJizzCount > 0 && searchStore.maxJizzCount > 0">
                {{ searchStore.minJizzCount }}-{{ searchStore.maxJizzCount }}
            </template>
            <template v-else-if="searchStore.minJizzCount > 0">
                {{ searchStore.minJizzCount }}+
            </template>
            <template v-else> &lt;{{ searchStore.maxJizzCount }} </template>
            <button
                class="text-dim hover:text-white"
                @click="
                    searchStore.minJizzCount = 0;
                    searchStore.maxJizzCount = 0;
                "
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.sort === 'random'"
            class="bg-lava/10 text-lava inline-flex items-center gap-1 rounded-full px-2.5 py-0.5
                text-[11px] font-medium"
        >
            Random
            <button class="hover:text-white" title="Reshuffle" @click="searchStore.reshuffle()">
                <Icon name="heroicons:arrow-path" size="11" />
            </button>
            <button class="hover:text-white" @click="searchStore.sort = ''">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.matchType !== 'broad'"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            {{ searchStore.matchType === 'strict' ? 'Strict Match' : 'Frequency Match' }}
            <button class="text-dim hover:text-white" @click="searchStore.matchType = 'broad'">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>
    </div>
</template>
