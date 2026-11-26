<script setup lang="ts">
import type { NavItem } from '~/utils/navItems';

// A link of the main nav row: icon only on mobile, icon + label from sm, with an optional
// count pill (e.g. the public lobbies next to Together).
defineProps<{ item: NavItem; badge?: string; badgeTitle?: string }>();
</script>

<template>
    <NuxtLink
        :to="item.to"
        class="text-dim relative flex items-center justify-center rounded-md px-2 py-1.5 text-[11px]
            font-medium transition-all hover:bg-white/5 hover:text-white sm:justify-start sm:gap-1.5
            sm:px-2.5 sm:py-1"
        active-class="!text-lava bg-lava/10"
        :title="badgeTitle ? `${item.label} (${badgeTitle})` : item.label"
    >
        <Icon :name="item.icon" size="18" class="sm:!h-3.5 sm:!w-3.5" />
        <span class="hidden sm:inline">{{ item.label }}</span>
        <span
            v-if="badge"
            class="border-lava/30 bg-lava/15 text-lava absolute top-0 right-0.5 flex h-3.5 min-w-3.5
                items-center justify-center rounded-full border px-1 font-mono text-[9px]
                leading-none font-semibold sm:static sm:h-4 sm:min-w-4"
            :aria-label="badgeTitle"
        >
            {{ badge }}
        </span>
    </NuxtLink>
</template>
