<script setup lang="ts">
import type { ProfileBlock } from '~/types/profile';

// most_watched: the most watched videos of the period, with their views (profile block).
defineProps<{ block: ProfileBlock; owner: boolean }>();
</script>

<template>
    <ProfileScenesGrid :scenes="block.scenes ?? []" figure="views" />
</template>
