<script setup lang="ts">
import type { ProfileBlock } from '~/types/profile';

// recent_markers: the markers the owner created last (profile block).
defineProps<{ block: ProfileBlock; owner: boolean }>();
</script>

<template>
    <ProfileMarkersGrid :markers="block.markers ?? []" />
</template>
