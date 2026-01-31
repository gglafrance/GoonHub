<script setup lang="ts">
const activeTab = ref<'jobs' | 'thumbnail' | 'details' | 'history' | 'markers'>('details');
</script>

<template>
    <div class="glass-panel overflow-hidden">
        <!-- Tab navigation -->
        <div class="border-border flex items-center gap-1 border-b px-4 pt-3 pb-0">
            <button
                @click="activeTab = 'details'"
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'details'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
            >
                Details
            </button>
            <button
                @click="activeTab = 'thumbnail'"
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'thumbnail'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
            >
                Thumbnail
            </button>
            <button
                @click="activeTab = 'jobs'"
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'jobs'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
            >
                Jobs
            </button>
            <button
                @click="activeTab = 'history'"
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'history'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
            >
                History
            </button>
            <button
                @click="activeTab = 'markers'"
                :class="[
                    'border-b-2 px-3 pb-2.5 text-[11px] font-medium transition-colors',
                    activeTab === 'markers'
                        ? 'border-lava text-white'
                        : 'text-dim border-transparent hover:text-white',
                ]"
            >
                Markers
            </button>
        </div>

        <!-- Tab content -->
        <div class="p-4">
            <WatchDetails v-if="activeTab === 'details'" />
            <WatchThumbnail v-if="activeTab === 'thumbnail'" />
            <WatchJobs v-if="activeTab === 'jobs'" />
            <WatchHistory v-if="activeTab === 'history'" />
            <WatchMarkers v-if="activeTab === 'markers'" />
        </div>
    </div>
</template>
