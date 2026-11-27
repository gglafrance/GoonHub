<script setup lang="ts">
import type { Profile } from '~/types/profile';

// Tabs of a profile page (Letterboxd style), in the owner's order: the profile, the list tabs
// and the custom tabs the viewer may open (`profile.tabs`, decided by the server).
const props = defineProps<{ profile: Profile }>();

const route = useRoute();

const tabs = computed(() =>
    (props.profile.tabs ?? []).map((t) => ({
        ...t,
        icon: t.icon || 'heroicons:squares-2x2',
        to: tabPath(props.profile.user.username, t),
    })),
);

const active = computed(() => activeTabId(route.path, props.profile.tabs ?? []));
</script>

<template>
    <nav v-if="tabs.length > 1" class="border-border border-b" aria-label="Profile sections">
        <div class="scrollbar-none -mb-px flex gap-1 overflow-x-auto overflow-y-hidden">
            <NuxtLink
                v-for="tab in tabs"
                :key="tab.id"
                :to="tab.to"
                class="flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-xs font-medium
                    transition-colors"
                :class="
                    active === tab.id
                        ? 'border-lava text-lava'
                        : 'text-dim border-transparent hover:text-white'
                "
                :aria-current="active === tab.id ? 'page' : undefined"
            >
                <Icon :name="tab.icon" size="14" />
                {{ tab.title }}
            </NuxtLink>
        </div>
    </nav>
</template>
