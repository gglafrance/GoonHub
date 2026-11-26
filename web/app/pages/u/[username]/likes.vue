<script setup lang="ts">
import type { Profile } from '~/types/profile';

// Likes tab of a profile page: every video the user liked (privacy item `liked`).
const props = defineProps<{ profile: Profile; owner: boolean }>();

definePageMeta({
    middleware: ['auth'],
});

useHead({ title: () => `Likes · ${displayNameOf(props.profile.user)}` });

const SORTS = [
    { value: 'liked_desc', label: 'Recently liked' },
    { value: 'liked_asc', label: 'First liked' },
];
</script>

<template>
    <section class="glass-panel p-4">
        <ProfileSceneList
            v-if="props.profile.lists?.liked"
            :username="props.profile.user.username"
            kind="likes"
            :sort-options="SORTS"
            figure="at"
            at-label="Liked"
            :empty-text="props.owner ? 'You have not liked any video yet.' : 'No liked video yet.'"
        />
        <ProfileListLocked v-else />
    </section>
</template>
