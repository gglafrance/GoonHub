<script setup lang="ts">
type DuplicatesSubTabType = 'config' | 'groups';

const props = defineProps<{
    activeSubTab: DuplicatesSubTabType;
}>();
</script>

<template>
    <div class="space-y-5">
        <SettingsDuplicatesConfig v-if="props.activeSubTab === 'config'" />
        <SettingsDuplicatesGroups v-if="props.activeSubTab === 'groups'" />
    </div>
</template>
