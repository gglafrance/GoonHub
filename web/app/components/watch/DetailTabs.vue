<script setup lang="ts">
const activeTab = ref<'jobs'>('jobs');
</script>

<template>
    <div class="glass-panel overflow-hidden">
        <!-- Tab navigation -->
        <div class="border-border flex items-center gap-1 border-b px-4 pt-3 pb-0">
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
        </div>

        <!-- Tab content -->
        <div class="p-4">
            <WatchJobs v-if="activeTab === 'jobs'" />
        </div>
    </div>
</template>
