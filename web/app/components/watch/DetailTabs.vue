<script setup lang="ts">
import type { Component } from 'vue';
import WatchDetails from './Details.vue';
import WatchThumbnail from './Thumbnail.vue';
import WatchJobs from './Jobs.vue';
import WatchHistory from './History.vue';
import WatchMarkers from './Markers.vue';
import WatchSimilar from './Similar.vue';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

type TabType = 'jobs' | 'thumbnail' | 'details' | 'history' | 'markers' | 'similar';

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
    similar: WatchSimilar,
};

// Thumbnail edits need scenes:edit; the Jobs tab uses the job admin routes (jobs:manage)
const perms = useMyPermissions();
const canEdit = computed(() => perms.has('scenes:edit'));
const canManageJobs = computed(() => perms.has('jobs:manage'));
const tabAllowed = (tab: TabType) =>
    (tab !== 'thumbnail' || canEdit.value) && (tab !== 'jobs' || canManageJobs.value);
watch(
    () => [activeTab.value, canEdit.value, canManageJobs.value] as const,
    ([tab]) => {
        if (!tabAllowed(tab)) activeTab.value = 'details';
    },
    { immediate: true },
);

const currentComponent = computed(() =>
    tabAllowed(activeTab.value) ? tabComponentMap[activeTab.value] : WatchDetails,
);

const watchPageData = inject<WatchPageData | undefined>(WATCH_PAGE_DATA_KEY, undefined);

// The Similar tab (scenes matching this one by fingerprint) only shows when there are some
const similarCount = computed(() => watchPageData?.similar.value?.scenes.length ?? 0);
watch(
    () => [similarCount.value, watchPageData?.loading.related] as const,
    ([count, loadingSimilar]) => {
        if (activeTab.value === 'similar' && count === 0 && !loadingSimilar) {
            activeTab.value = 'details';
        }
    },
);

// Markers tab label with the scene's marker count, live as markers are added or removed
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
                v-if="canEdit"
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
                v-if="canManageJobs"
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
            <button
                v-if="similarCount > 0"
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'similar'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
                title="Scenes matching this one by audio or visual fingerprint"
                @click="activeTab = 'similar'"
            >
                Similar ({{ similarCount }})
            </button>
        </div>

        <!-- Tab content - KeepAlive caches component state to avoid re-fetching data on tab switch -->
        <div class="p-4">
            <KeepAlive>
                <component
                    :is="currentComponent"
                    :key="tabAllowed(activeTab) ? activeTab : 'details'"
                />
            </KeepAlive>
        </div>
    </div>
</template>
