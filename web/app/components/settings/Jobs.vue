<script setup lang="ts">
// Sub-tab id from the settings page (read from the URL): one of the Jobs sub-tabs of
// utils/settingsTabs.ts (manual, history, workers, scheduling, processing, triggers, retry, dlq,
// failing)
const props = defineProps<{
    activeSubTab: string;
}>();
</script>

<template>
    <div class="space-y-5">
        <!-- History sub-tab -->
        <SettingsJobsHistoryTab v-if="props.activeSubTab === 'history'" />

        <!-- Workers sub-tab -->
        <SettingsJobsWorkers v-if="props.activeSubTab === 'workers'" />

        <!-- Scheduling sub-tab -->
        <SettingsJobsScheduling v-if="props.activeSubTab === 'scheduling'" />

        <!-- Processing sub-tab -->
        <SettingsJobsProcessing v-if="props.activeSubTab === 'processing'" />

        <!-- Triggers sub-tab -->
        <SettingsJobsTriggers v-if="props.activeSubTab === 'triggers'" />

        <!-- Retry sub-tab -->
        <SettingsJobsRetry v-if="props.activeSubTab === 'retry'" />

        <!-- DLQ sub-tab -->
        <SettingsJobsDLQ v-if="props.activeSubTab === 'dlq'" />

        <!-- Failing sub-tab -->
        <SettingsJobsFailing v-if="props.activeSubTab === 'failing'" />

        <!-- Manual sub-tab -->
        <SettingsJobsManual v-if="props.activeSubTab === 'manual'" />
    </div>
</template>
