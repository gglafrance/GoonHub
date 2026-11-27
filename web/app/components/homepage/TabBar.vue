<script setup lang="ts">
import type { HomepageTab } from '~/types/homepage';

// Homepage tab bar (hidden when there is a single tab).
defineProps<{
    tabs: HomepageTab[];
    activeId: string | null;
}>();

const emit = defineEmits<{
    select: [tabId: string];
}>();
</script>

<template>
    <nav
        v-if="tabs.length > 1"
        class="border-border bg-void/70 sticky top-[var(--app-header-h)] z-30 -mx-4 mb-4 flex gap-1
            overflow-x-auto border-b px-4 py-2 backdrop-blur-md sm:mx-0 sm:rounded-lg sm:border
            sm:px-2"
        aria-label="Homepage tabs"
    >
        <button
            v-for="tab in tabs"
            :key="tab.id"
            class="flex shrink-0 items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium
                transition-all"
            :class="
                tab.id === activeId
                    ? 'bg-lava/15 text-lava'
                    : 'text-dim hover:bg-white/5 hover:text-white'
            "
            :aria-current="tab.id === activeId ? 'page' : undefined"
            @click="emit('select', tab.id)"
        >
            <Icon v-if="tab.icon" :name="tab.icon" size="14" />
            {{ tab.name }}
        </button>
        <NuxtLink
            to="/settings?tab=homepage"
            class="text-dim ml-auto flex shrink-0 items-center rounded-md px-2 transition-colors
                hover:text-white"
            title="Customize tabs"
        >
            <Icon name="heroicons:adjustments-horizontal" size="14" />
        </NuxtLink>
    </nav>
</template>
