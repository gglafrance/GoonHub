<script setup lang="ts">
import type { Profile } from '~/types/profile';

// Profile tab of a profile page: its sections and blocks (the parent page loads the profile
// with this tab and passes it).
const props = defineProps<{ profile: Profile; owner: boolean }>();

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <ProfileSections :sections="props.profile.sections ?? []" :owner="props.owner" />
</template>
