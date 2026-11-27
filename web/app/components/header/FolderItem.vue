<script setup lang="ts">
import type { NavItemDef } from '~/utils/navRegistry';

// A row of a folder menu: a page link (with its live badge) or an action button (palette,
// shortcuts, safe mode, preview sound, logout). Emits select when activated.
const props = defineProps<{ item: NavItemDef }>();
const emit = defineEmits<{ select: [] }>();

const route = useRoute();
const { badge: badgeFor, route: routeFor, menuRow, run } = useHeaderItems();

const to = computed(() => routeFor(props.item));
const row = computed(() => menuRow(props.item));
const badge = computed(() => badgeFor(props.item.id));
const active = computed(() => !!to.value && isNavItemActive(route.path, to.value));

function onAction() {
    emit('select');
    run(props.item);
}

const rowClass = `text-dim focus-visible:ring-lava/40 flex w-full items-center gap-2 rounded-md
    px-2.5 py-1.5 text-left text-[11px] font-medium transition-colors outline-none hover:bg-white/5
    hover:text-white focus-visible:bg-white/5 focus-visible:text-white focus-visible:ring-1`;
</script>

<template>
    <NuxtLink
        v-if="to"
        :to="to"
        role="menuitem"
        :class="[rowClass, { '!text-lava bg-lava/10': active }]"
        :title="badge ? `${row.label} (${badge.title})` : undefined"
        @click="emit('select')"
    >
        <Icon :name="row.icon" size="14" class="shrink-0" />
        <span class="flex-1 truncate">{{ row.label }}</span>
        <span
            v-if="badge"
            class="border-lava/30 bg-lava/15 text-lava flex h-4 min-w-4 items-center justify-center
                rounded-full border px-1 font-mono text-[9px] leading-none font-semibold"
            :aria-label="badge.title"
        >
            {{ badge.text }}
        </span>
    </NuxtLink>
    <button v-else type="button" role="menuitem" :class="rowClass" @click="onAction">
        <Icon :name="row.icon" size="14" class="shrink-0" />
        <span class="flex-1 truncate">{{ row.label }}</span>
    </button>
</template>
