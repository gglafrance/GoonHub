<script setup lang="ts">
const searchStore = useSearchStore();

const removeTag = (tag: string) => {
    searchStore.selectedTags = searchStore.selectedTags.filter((t) => t !== tag);
};

const removeActor = (actor: string) => {
    searchStore.selectedActors = searchStore.selectedActors.filter((a) => a !== actor);
};

const removeMarkerLabel = (label: string) => {
    searchStore.selectedMarkerLabels = searchStore.selectedMarkerLabels.filter((l) => l !== label);
};
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
            <button @click="searchStore.query = ''" class="hover:text-white">
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
            <button @click="removeTag(tag)" class="text-dim hover:text-white">
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
            <button @click="removeActor(actor)" class="text-dim hover:text-white">
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
            <button @click="removeMarkerLabel(label)" class="text-dim hover:text-white">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.studio"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            Studio: {{ searchStore.studio }}
            <button @click="searchStore.studio = ''" class="text-dim hover:text-white">
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
                @click="
                    searchStore.minDuration = 0;
                    searchStore.maxDuration = 0;
                "
                class="text-dim hover:text-white"
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
            <button @click="searchStore.resolution = ''" class="text-dim hover:text-white">
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
                @click="
                    searchStore.minDate = '';
                    searchStore.maxDate = '';
                "
                class="text-dim hover:text-white"
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
            <button @click="searchStore.liked = false" class="text-dim hover:text-white">
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
                @click="
                    searchStore.minRating = 0;
                    searchStore.maxRating = 0;
                "
                class="text-dim hover:text-white"
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
                @click="
                    searchStore.minJizzCount = 0;
                    searchStore.maxJizzCount = 0;
                "
                class="text-dim hover:text-white"
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>

        <span
            v-if="searchStore.matchType !== 'broad'"
            class="inline-flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-[11px]
                font-medium text-white"
        >
            {{ searchStore.matchType === 'strict' ? 'Strict Match' : 'Frequency Match' }}
            <button @click="searchStore.matchType = 'broad'" class="text-dim hover:text-white">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </span>
    </div>
</template>
