<script setup lang="ts">
type JobsSubTabType =
    | 'manual'
    | 'history'
    | 'workers'
    | 'processing'
    | 'triggers'
    | 'retry'
    | 'dlq';

const props = defineProps<{
    activeSubTab: JobsSubTabType;
}>();
</script>

<template>
    <div class="space-y-5">
        <!-- History sub-tab -->
        <SettingsJobsHistoryTab v-if="props.activeSubTab === 'history'" />

        <!-- Workers sub-tab -->
        <SettingsJobsWorkers v-if="props.activeSubTab === 'workers'" />

        <!-- Processing sub-tab -->
        <SettingsJobsProcessing v-if="props.activeSubTab === 'processing'" />

        <!-- Triggers sub-tab -->
        <SettingsJobsTriggers v-if="props.activeSubTab === 'triggers'" />

        <!-- Retry sub-tab -->
        <SettingsJobsRetry v-if="props.activeSubTab === 'retry'" />

        <!-- DLQ sub-tab -->
        <SettingsJobsDLQ v-if="props.activeSubTab === 'dlq'" />

        <!-- Manual sub-tab -->
        <SettingsJobsManual v-if="props.activeSubTab === 'manual'" />
    </div>
</template>
