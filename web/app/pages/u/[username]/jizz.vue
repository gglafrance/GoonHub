<script setup lang="ts">
import type { Profile } from '~/types/profile';

// Jizz tab of a profile page: the videos the user came to, with their O count and the last
// one (privacy item `jizz`).
const props = defineProps<{ profile: Profile; owner: boolean }>();

definePageMeta({
    middleware: ['auth'],
});

useHead({ title: () => `Jizz · ${displayNameOf(props.profile.user)}` });

const SORTS = [
    { value: 'recent', label: 'Most recent' },
    { value: 'count_desc', label: 'Most O' },
];
</script>

<template>
    <section class="glass-panel p-4">
        <ProfileSceneList
            v-if="props.profile.lists?.jizz"
            :username="props.profile.user.username"
            kind="jizz"
            :sort-options="SORTS"
            figure="jizz"
            :empty-text="props.owner ? 'No O recorded yet.' : 'Nothing here yet.'"
        />
        <ProfileListLocked v-else />
    </section>
</template>
