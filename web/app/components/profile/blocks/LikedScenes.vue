<script setup lang="ts">
import type { ProfileBlock } from '~/types/profile';

// liked_scenes: the videos liked last (profile block).
defineProps<{ block: ProfileBlock; owner: boolean }>();
</script>

<template>
    <ProfileScenesGrid :scenes="block.scenes ?? []" figure="at" at-label="Liked" />
</template>
