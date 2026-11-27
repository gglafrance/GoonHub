<script setup lang="ts">
import type { NavItemDef } from '~/utils/navRegistry';

// A page of the mobile Menu sheet: icon tile with its label and live badge.
const props = defineProps<{ item: NavItemDef }>();

const route = useRoute();
const { badge: badgeFor } = useHeaderItems();
const badge = computed(() => badgeFor(props.item.id));
const active = computed(() => !!props.item.to && isNavItemActive(route.path, props.item.to));
</script>

<template>
    <NuxtLink
        :to="item.to!"
        class="relative flex min-h-[4.5rem] flex-col items-center justify-center gap-1.5 rounded-xl
            border px-1 py-2.5 text-center transition-colors"
        :class="
            active
                ? 'border-lava/40 bg-lava/10 text-lava'
                : 'border-border bg-panel/70 text-white/80 active:bg-white/5'
        "
        :aria-current="active ? 'page' : undefined"
    >
        <Icon :name="item.icon" size="22" :class="active ? '' : 'text-white/60'" />
        <span class="w-full truncate text-[11px] leading-tight font-medium">{{ item.label }}</span>
        <span
            v-if="badge"
            class="border-lava/30 bg-lava/15 text-lava absolute top-1.5 right-1.5 flex h-4 min-w-4
                items-center justify-center rounded-full border px-1 font-mono text-[9px]
                leading-none font-semibold"
            :aria-label="badge.title"
        >
            {{ badge.text }}
        </span>
    </NuxtLink>
</template>
