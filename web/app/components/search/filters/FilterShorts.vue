<script setup lang="ts">
import { SHORTS_SEARCH_OPTIONS, isShortsSearchFilter } from '~/types/shorts';

// Shorts are scenes from 1s up to the Shorts length limit, plus shorts cut
// from another scene at any length. Created clips are only the cut ones.
const searchStore = useSearchStore();

const badges: Record<string, string> = {
    all: '',
    only: 'only',
    hide: 'hide',
    only_clips: 'clips only',
    hide_clips: 'no clips',
};

const value = computed({
    get: () => searchStore.shorts,
    set: (v: string) => {
        searchStore.shorts = isShortsSearchFilter(v) ? v : searchStore.shortsDefault;
    },
});
</script>

<template>
    <SearchFiltersFilterSelect
        v-model="value"
        title="Shorts"
        icon="heroicons:device-phone-mobile"
        :options="SHORTS_SEARCH_OPTIONS"
        :badge-text="badges[value]"
        default-collapsed
    />
</template>
