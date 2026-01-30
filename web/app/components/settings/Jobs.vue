<script setup lang="ts">
const activeSubTab = ref<
    'manual' | 'history' | 'workers' | 'processing' | 'triggers' | 'retry' | 'dlq'
>('manual');
</script>

<template>
    <div class="space-y-5">
        <!-- Sub-tab navigation -->
        <div class="flex items-center gap-1">
            <button
                @click="activeSubTab = 'manual'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'manual'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Manual
            </button>
            <button
                @click="activeSubTab = 'history'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'history'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                History
            </button>
            <button
                @click="activeSubTab = 'workers'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'workers'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Workers
            </button>
            <button
                @click="activeSubTab = 'processing'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'processing'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Processing
            </button>
            <button
                @click="activeSubTab = 'triggers'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'triggers'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Triggers
            </button>
            <button
                @click="activeSubTab = 'retry'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'retry'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Retry
            </button>
            <button
                @click="activeSubTab = 'dlq'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'dlq' ? 'bg-white/10 text-white' : 'text-dim hover:text-white',
                ]"
            >
                DLQ
            </button>
        </div>

        <!-- History sub-tab -->
        <SettingsJobsHistoryTab v-if="activeSubTab === 'history'" />

        <!-- Workers sub-tab -->
        <SettingsJobsWorkers v-if="activeSubTab === 'workers'" />

        <!-- Processing sub-tab -->
        <SettingsJobsProcessing v-if="activeSubTab === 'processing'" />

        <!-- Triggers sub-tab -->
        <SettingsJobsTriggers v-if="activeSubTab === 'triggers'" />

        <!-- Retry sub-tab -->
        <SettingsJobsRetry v-if="activeSubTab === 'retry'" />

        <!-- DLQ sub-tab -->
        <SettingsJobsDLQ v-if="activeSubTab === 'dlq'" />

        <!-- Manual sub-tab -->
        <SettingsJobsManual v-if="activeSubTab === 'manual'" />
    </div>
</template>
