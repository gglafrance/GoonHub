<script setup lang="ts">
import type { Component } from 'vue';
import WatchDetails from './Details.vue';
import WatchThumbnail from './Thumbnail.vue';
import WatchJobs from './Jobs.vue';
import WatchHistory from './History.vue';
import WatchMarkers from './Markers.vue';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

type TabType = 'jobs' | 'thumbnail' | 'details' | 'history' | 'markers';

// Inject activeTab from parent (watch page) or use local state
const injectedActiveTab = inject<Ref<TabType> | undefined>('activeTab', undefined);
const localActiveTab = ref<TabType>('details');
const activeTab = injectedActiveTab ?? localActiveTab;

// Map tab names to components for KeepAlive caching
const tabComponentMap: Record<TabType, Component> = {
    details: WatchDetails,
    thumbnail: WatchThumbnail,
    jobs: WatchJobs,
    history: WatchHistory,
    markers: WatchMarkers,
};

const currentComponent = computed(() => tabComponentMap[activeTab.value]);

// Markers tab label with the scene's marker count, live as markers are added or removed
const watchPageData = inject<WatchPageData | undefined>(WATCH_PAGE_DATA_KEY, undefined);
const markersTabLabel = computed(() => markerTabLabel(watchPageData?.markers.value.length ?? 0));
</script>

<template>
    <div class="glass-panel overflow-hidden">
        <!-- Tab navigation -->
        <div class="border-border flex items-center gap-1 border-b px-4 pt-3 pb-0">
            <button
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'details'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
                @click="activeTab = 'details'"
            >
                Details
            </button>
            <button
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'thumbnail'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
                @click="activeTab = 'thumbnail'"
            >
                Thumbnail
            </button>
            <button
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'jobs'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
                @click="activeTab = 'jobs'"
            >
                Jobs
            </button>
            <button
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'history'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
                @click="activeTab = 'history'"
            >
                History
            </button>
            <button
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'markers'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
                @click="activeTab = 'markers'"
            >
                {{ markersTabLabel }}
            </button>
        </div>

        <!-- Tab content - KeepAlive caches component state to avoid re-fetching data on tab switch -->
        <div class="p-4">
            <KeepAlive>
                <component :is="currentComponent" :key="activeTab" />
            </KeepAlive>
        </div>
    </div>
</template>
