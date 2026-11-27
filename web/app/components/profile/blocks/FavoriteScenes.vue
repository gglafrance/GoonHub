<script setup lang="ts">
import type { ProfileBlock } from '~/types/profile';

// favorite_scenes: the favorite videos showcase, in the owner's order (profile block).
defineProps<{ block: ProfileBlock; owner: boolean }>();
</script>

<template>
    <ProfileScenesGrid :scenes="block.scenes ?? []" figure="none" showcase />
</template>
