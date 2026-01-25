<script setup lang="ts">
const searchStore = useSearchStore();

const showFilters = ref(true);

const collapsed = ref<Record<string, boolean>>({
    tags: false,
    actors: false,
    studio: true,
    duration: false,
    date: true,
    resolution: true,
    liked: true,
    rating: true,
    jizzCount: true,
});

const toggle = (section: string) => {
    collapsed.value[section] = !collapsed.value[section];
};

const resolutionOptions = [
    { value: '', label: 'Any' },
    { value: '4k', label: '4K' },
    { value: '1440p', label: '1440p' },
    { value: '1080p', label: '1080p' },
    { value: '720p', label: '720p' },
    { value: '480p', label: '480p' },
    { value: '360p', label: '360p' },
];

const toggleTag = (tagName: string) => {
    const idx = searchStore.selectedTags.indexOf(tagName);
    if (idx >= 0) {
        searchStore.selectedTags.splice(idx, 1);
    } else {
        searchStore.selectedTags.push(tagName);
    }
};

const toggleActor = (actor: string) => {
    const idx = searchStore.selectedActors.indexOf(actor);
    if (idx >= 0) {
        searchStore.selectedActors.splice(idx, 1);
    } else {
        searchStore.selectedActors.push(actor);
    }
};

const durationPresets = [
    { label: '5+ min', min: 300 },
    { label: '10+ min', min: 600 },
    { label: '20+ min', min: 1200 },
    { label: '30+ min', min: 1800 },
    { label: '60+ min', min: 3600 },
];

const setDuration = (min: number) => {
    if (searchStore.minDuration === min) {
        searchStore.minDuration = 0;
    } else {
        searchStore.minDuration = min;
    }
    searchStore.maxDuration = 0;
};
</script>

<template>
    <aside>
        <div class="mb-3 flex items-center justify-between">
            <h3 class="text-xs font-semibold tracking-wide text-white uppercase">Filters</h3>
            <button
                @click="showFilters = !showFilters"
                class="text-dim transition-colors hover:text-white"
            >
                <Icon
                    :name="showFilters ? 'heroicons:chevron-up' : 'heroicons:chevron-down'"
                    size="14"
                />
            </button>
        </div>

        <div v-show="showFilters" class="space-y-1">
            <!-- Tags -->
            <div
                v-if="searchStore.filterOptions.tags.length > 0"
                class="border-border rounded-md border"
            >
                <button
                    @click="toggle('tags')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Tags</span>
                    <div class="flex items-center gap-1.5">
                        <span
                            v-if="searchStore.selectedTags.length"
                            class="bg-lava/20 text-lava rounded-full px-1.5 text-[10px] font-medium"
                        >
                            {{ searchStore.selectedTags.length }}
                        </span>
                        <Icon
                            :name="collapsed.tags ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.tags" class="border-border border-t px-2.5 pb-2.5">
                    <div class="max-h-40 space-y-1 overflow-y-auto pt-2">
                        <button
                            v-for="tag in searchStore.filterOptions.tags"
                            :key="tag.id"
                            @click="toggleTag(tag.name)"
                            class="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-xs
                                transition-colors"
                            :class="
                                searchStore.selectedTags.includes(tag.name)
                                    ? 'bg-lava/10 text-white'
                                    : 'text-dim hover:bg-white/5 hover:text-white'
                            "
                        >
                            <span
                                class="h-2 w-2 shrink-0 rounded-full"
                                :style="{ backgroundColor: tag.color }"
                            ></span>
                            <span class="flex-1 truncate">{{ tag.name }}</span>
                            <span class="text-[10px] opacity-50">{{ tag.video_count }}</span>
                        </button>
                    </div>
                </div>
            </div>

            <!-- Actors -->
            <div
                v-if="searchStore.filterOptions.actors.length > 0"
                class="border-border rounded-md border"
            >
                <button
                    @click="toggle('actors')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Actors</span>
                    <div class="flex items-center gap-1.5">
                        <span
                            v-if="searchStore.selectedActors.length"
                            class="bg-lava/20 text-lava rounded-full px-1.5 text-[10px] font-medium"
                        >
                            {{ searchStore.selectedActors.length }}
                        </span>
                        <Icon
                            :name="collapsed.actors ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.actors" class="border-border border-t px-2.5 pb-2.5">
                    <div class="max-h-40 space-y-1 overflow-y-auto pt-2">
                        <button
                            v-for="actor in searchStore.filterOptions.actors"
                            :key="actor"
                            @click="toggleActor(actor)"
                            class="w-full rounded px-2 py-1 text-left text-xs transition-colors"
                            :class="
                                searchStore.selectedActors.includes(actor)
                                    ? 'bg-lava/10 text-white'
                                    : 'text-dim hover:bg-white/5 hover:text-white'
                            "
                        >
                            {{ actor }}
                        </button>
                    </div>
                </div>
            </div>

            <!-- Studio -->
            <div
                v-if="searchStore.filterOptions.studios.length > 0"
                class="border-border rounded-md border"
            >
                <button
                    @click="toggle('studio')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Studio</span>
                    <div class="flex items-center gap-1.5">
                        <span
                            v-if="searchStore.studio"
                            class="text-lava max-w-20 truncate text-[10px]"
                        >
                            {{ searchStore.studio }}
                        </span>
                        <Icon
                            :name="collapsed.studio ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.studio" class="border-border border-t px-2.5 py-2.5">
                    <select
                        v-model="searchStore.studio"
                        class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                            text-xs focus:border-white/20 focus:outline-none"
                    >
                        <option value="">All Studios</option>
                        <option v-for="s in searchStore.filterOptions.studios" :key="s" :value="s">
                            {{ s }}
                        </option>
                    </select>
                </div>
            </div>

            <!-- Duration -->
            <div class="border-border rounded-md border">
                <button
                    @click="toggle('duration')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Duration</span>
                    <div class="flex items-center gap-1.5">
                        <span
                            v-if="searchStore.minDuration || searchStore.maxDuration"
                            class="text-lava text-[10px]"
                        >
                            <template v-if="searchStore.minDuration && searchStore.maxDuration">
                                {{ Math.floor(searchStore.minDuration / 60) }}-{{ Math.floor(searchStore.maxDuration / 60) }}m
                            </template>
                            <template v-else-if="searchStore.minDuration">
                                {{ Math.floor(searchStore.minDuration / 60) }}+ min
                            </template>
                            <template v-else>
                                &lt;{{ Math.floor(searchStore.maxDuration / 60) }}m
                            </template>
                        </span>
                        <Icon
                            :name="collapsed.duration ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.duration" class="border-border border-t px-2.5 py-2.5">
                    <div class="grid grid-cols-2 gap-1.5">
                        <button
                            v-for="preset in durationPresets"
                            :key="preset.label"
                            @click="setDuration(preset.min)"
                            class="rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors"
                            :class="
                                searchStore.minDuration === preset.min && !searchStore.maxDuration
                                    ? 'bg-lava/10 text-white border border-lava/30'
                                    : 'border-border bg-surface text-dim border hover:text-white hover:border-white/20'
                            "
                        >
                            {{ preset.label }}
                        </button>
                    </div>
                    <div class="mt-2 flex items-center gap-1.5">
                        <input
                            :value="searchStore.minDuration ? Math.floor(searchStore.minDuration / 60) : ''"
                            @input="searchStore.minDuration = ($event.target as HTMLInputElement).value ? Number(($event.target as HTMLInputElement).value) * 60 : 0"
                            type="number"
                            min="0"
                            placeholder="Min"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                        <span class="text-dim text-[10px]">-</span>
                        <input
                            :value="searchStore.maxDuration ? Math.floor(searchStore.maxDuration / 60) : ''"
                            @input="searchStore.maxDuration = ($event.target as HTMLInputElement).value ? Number(($event.target as HTMLInputElement).value) * 60 : 0"
                            type="number"
                            min="0"
                            placeholder="Max"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                        <span class="text-dim shrink-0 text-[10px]">min</span>
                    </div>
                </div>
            </div>

            <!-- Date Range -->
            <div class="border-border rounded-md border">
                <button
                    @click="toggle('date')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Date</span>
                    <div class="flex items-center gap-1.5">
                        <span
                            v-if="searchStore.minDate || searchStore.maxDate"
                            class="text-lava text-[10px]"
                        >
                            {{ searchStore.minDate || '*' }} - {{ searchStore.maxDate || '*' }}
                        </span>
                        <Icon
                            :name="collapsed.date ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.date" class="border-border border-t px-2.5 py-2.5">
                    <div class="space-y-1.5">
                        <input
                            v-model="searchStore.minDate"
                            type="date"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                        <input
                            v-model="searchStore.maxDate"
                            type="date"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                    </div>
                </div>
            </div>

            <!-- Resolution -->
            <div class="border-border rounded-md border">
                <button
                    @click="toggle('resolution')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Resolution</span>
                    <div class="flex items-center gap-1.5">
                        <span v-if="searchStore.resolution" class="text-lava text-[10px]">
                            {{ searchStore.resolution }}
                        </span>
                        <Icon
                            :name="collapsed.resolution ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.resolution" class="border-border border-t px-2.5 py-2.5">
                    <select
                        v-model="searchStore.resolution"
                        class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                            text-xs focus:border-white/20 focus:outline-none"
                    >
                        <option v-for="opt in resolutionOptions" :key="opt.value" :value="opt.value">
                            {{ opt.label }}
                        </option>
                    </select>
                </div>
            </div>

            <!-- Liked -->
            <div class="border-border rounded-md border">
                <button
                    @click="toggle('liked')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Liked</span>
                    <div class="flex items-center gap-1.5">
                        <span v-if="searchStore.liked" class="text-lava text-[10px]">Yes</span>
                        <Icon
                            :name="collapsed.liked ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.liked" class="border-border border-t px-2.5 py-2.5">
                    <label class="flex cursor-pointer items-center gap-2">
                        <input
                            v-model="searchStore.liked"
                            type="checkbox"
                            class="accent-lava h-3.5 w-3.5 rounded"
                        />
                        <span class="text-dim text-xs">Only show liked videos</span>
                    </label>
                </div>
            </div>

            <!-- Rating -->
            <div class="border-border rounded-md border">
                <button
                    @click="toggle('rating')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Rating</span>
                    <div class="flex items-center gap-1.5">
                        <span
                            v-if="searchStore.minRating > 0 || searchStore.maxRating > 0"
                            class="text-lava text-[10px]"
                        >
                            <template v-if="searchStore.minRating && searchStore.maxRating">
                                {{ searchStore.minRating }}-{{ searchStore.maxRating }}
                            </template>
                            <template v-else-if="searchStore.minRating">
                                {{ searchStore.minRating }}+
                            </template>
                            <template v-else>
                                &lt;{{ searchStore.maxRating }}
                            </template>
                        </span>
                        <Icon
                            :name="collapsed.rating ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.rating" class="border-border border-t px-2.5 py-2.5">
                    <div class="flex items-center gap-1.5">
                        <input
                            v-model.number="searchStore.minRating"
                            type="number"
                            min="0"
                            max="5"
                            step="0.5"
                            placeholder="Min"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                        <span class="text-dim text-[10px]">-</span>
                        <input
                            v-model.number="searchStore.maxRating"
                            type="number"
                            min="0"
                            max="5"
                            step="0.5"
                            placeholder="Max"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                    </div>
                    <p class="text-dim mt-1.5 text-[10px]">Scale: 0.5 - 5</p>
                </div>
            </div>

            <!-- Jizz Count -->
            <div class="border-border rounded-md border">
                <button
                    @click="toggle('jizzCount')"
                    class="flex w-full items-center justify-between px-2.5 py-2"
                >
                    <span class="text-dim text-[11px] font-medium uppercase">Jizz Count</span>
                    <div class="flex items-center gap-1.5">
                        <span
                            v-if="searchStore.minJizzCount > 0 || searchStore.maxJizzCount > 0"
                            class="text-lava text-[10px]"
                        >
                            <template v-if="searchStore.minJizzCount && searchStore.maxJizzCount">
                                {{ searchStore.minJizzCount }}-{{ searchStore.maxJizzCount }}
                            </template>
                            <template v-else-if="searchStore.minJizzCount">
                                {{ searchStore.minJizzCount }}+
                            </template>
                            <template v-else>
                                &lt;{{ searchStore.maxJizzCount }}
                            </template>
                        </span>
                        <Icon
                            :name="collapsed.jizzCount ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                            size="12"
                            class="text-dim"
                        />
                    </div>
                </button>
                <div v-show="!collapsed.jizzCount" class="border-border border-t px-2.5 py-2.5">
                    <div class="flex items-center gap-1.5">
                        <input
                            v-model.number="searchStore.minJizzCount"
                            type="number"
                            min="0"
                            placeholder="Min"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                        <span class="text-dim text-[10px]">-</span>
                        <input
                            v-model.number="searchStore.maxJizzCount"
                            type="number"
                            min="0"
                            placeholder="Max"
                            class="border-border bg-surface text-dim w-full rounded-md border px-2
                                py-1.5 text-xs focus:border-white/20 focus:outline-none"
                        />
                    </div>
                </div>
            </div>

            <!-- Reset -->
            <button
                v-if="searchStore.hasActiveFilters"
                @click="
                    searchStore.resetFilters();
                    searchStore.search();
                "
                class="text-lava hover:text-lava/80 w-full rounded-md py-2 text-xs font-medium
                    transition-colors"
            >
                Reset All Filters
            </button>
        </div>
    </aside>
</template>
