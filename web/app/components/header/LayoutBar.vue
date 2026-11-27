<script setup lang="ts">
import type { NavBarId, NavZoneId } from '~/types/navLayout';
import type { ResolvedNavBar } from '~/utils/navLayout';

// One header bar laid out in three zones (left, center, right) from the user's layout
// (docs/NAVIGATION.md). From sm the left and right zones share the free space so that the
// center zone stays centered. On mobile the zones collapse: the top bar keeps left and center
// together at the start and right at the end; the nav row lays every entry out in one row
// (spread out, scrolling horizontally when it does not fit).
const props = defineProps<{ bar: NavBarId; zones: ResolvedNavBar }>();

const zoneClass = (zone: NavZoneId): string[] => {
    if (props.bar === 'top') {
        const base = ['flex', 'items-center', 'gap-2', 'whitespace-nowrap', 'sm:gap-3'];
        if (zone === 'left') return [...base, 'sm:flex-1'];
        if (zone === 'center') {
            return props.zones.center.length > 0 ? [...base, 'px-2', 'sm:px-3'] : base;
        }
        return [...base, 'ml-auto', 'sm:ml-0', 'sm:flex-1', 'sm:justify-end'];
    }
    const base = ['contents', 'sm:flex', 'sm:items-center', 'sm:gap-1', 'sm:whitespace-nowrap'];
    if (zone === 'left') return [...base, 'sm:flex-1'];
    if (zone === 'center') return [...base, 'sm:justify-center'];
    return [...base, 'sm:flex-1', 'sm:justify-end'];
};
</script>

<template>
    <div
        :class="
            bar === 'top'
                ? 'flex h-12 items-center'
                : `scrollbar-none flex h-10 items-center justify-between gap-1 overflow-x-auto
                    sm:h-9 sm:justify-start sm:overflow-visible`
        "
    >
        <div v-for="zone in NAV_ZONES" :key="zone" :class="zoneClass(zone)">
            <template v-for="entry in zones[zone]" :key="entry.key">
                <HeaderNavFolder
                    v-if="entry.kind === 'folder'"
                    :folder="entry.folder"
                    :items="entry.items"
                />
                <HeaderEntry v-else :item="entry.item" />
            </template>
        </div>
    </div>
</template>
