<script setup lang="ts">
import type { ProfileBlock } from '~/types/profile';

// favorite_markers: the favorite markers showcase, in the owner's order (profile block).
defineProps<{ block: ProfileBlock; owner: boolean }>();
</script>

<template>
    <ProfileMarkersGrid :markers="block.markers ?? []" showcase />
</template>
