<script setup lang="ts">
import type { Profile } from '~/types/profile';

// A custom tab of a profile page (/u/<name>/t/<slug>): its sections and blocks, loaded by the
// parent page with this tab.
const props = defineProps<{ profile: Profile; owner: boolean }>();

definePageMeta({
    middleware: ['auth'],
});

const title = computed(() => props.profile.tabs.find((t) => t.id === props.profile.tab)?.title);
useHead({ title: () => `${title.value ?? 'Tab'} · ${displayNameOf(props.profile.user)}` });
</script>

<template>
    <div
        v-if="props.profile.tab_missing"
        class="glass-panel text-dim flex items-center justify-center gap-1.5 p-8 text-xs"
    >
        <Icon name="heroicons:lock-closed" size="13" />
        This tab does not exist or is private.
    </div>
    <ProfileSections v-else :sections="props.profile.sections ?? []" :owner="props.owner" />
</template>
