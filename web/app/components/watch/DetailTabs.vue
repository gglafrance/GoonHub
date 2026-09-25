<script setup lang="ts">
import type { Component } from 'vue';
import WatchDetails from './Details.vue';
import WatchThumbnail from './Thumbnail.vue';
import WatchJobs from './Jobs.vue';
import WatchHistory from './History.vue';
import WatchMarkers from './Markers.vue';
import WatchShorts from './Shorts.vue';

type TabType = 'jobs' | 'thumbnail' | 'details' | 'history' | 'markers' | 'shorts';

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
    shorts: WatchShorts,
};

const currentComponent = computed(() => tabComponentMap[activeTab.value]);

// Number of shorts cut from this scene, for the tab badge
const route = useRoute();
const { getSceneShorts } = useApiShorts();
const shortTasks = useShortTasksStore();
const shortsCount = ref(0);
const tabSceneId = computed(() => parseInt(route.params.id as string));
const loadShortsCount = async () => {
    try {
        const res = await getSceneShorts(tabSceneId.value, 1, 1);
        shortsCount.value = res.total;
    } catch {
        shortsCount.value = 0;
    }
};
onMounted(loadShortsCount);
watch(tabSceneId, loadShortsCount);
watch(() => shortTasks.completedTick[tabSceneId.value], loadShortsCount);
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
                Markers
            </button>
            <button
                :class="[
                    'flex items-center gap-1.5 border-b-2 px-3 pb-2.5 text-[11px] font-medium',
                    'transition-colors',
                    activeTab === 'shorts'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
                @click="activeTab = 'shorts'"
            >
                Shorts
                <span
                    v-if="shortsCount > 0"
                    class="bg-lava/15 text-lava rounded-full px-1.5 font-mono text-[9px] leading-4"
                    >{{ shortsCount }}</span
                >
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
