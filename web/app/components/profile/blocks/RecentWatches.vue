<script setup lang="ts">
import type { ProfileBlock } from '~/types/profile';

// recent_watches: the videos watched last, with when (profile block).
defineProps<{ block: ProfileBlock; owner: boolean }>();
</script>

<template>
    <ProfileScenesGrid :scenes="block.scenes ?? []" figure="at" at-label="Watched" />
</template>
