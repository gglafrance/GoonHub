<script setup lang="ts">
import type { NavLayout } from '~/types/navLayout';

// Live preview of the draft layout with the real header bars (not interactive). The frame
// scrolls when the bars are wider than the settings column.
const props = defineProps<{ layout: NavLayout }>();
const perms = useMyPermissions();

const resolved = computed(() => resolveNavLayout(props.layout, perms.has));
const showNavRow = computed(() => !isNavBarEmpty(resolved.value.nav));
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-1 text-sm font-semibold text-white">Preview</h3>
        <p class="text-dim mb-3 text-[11px]">Your header with the changes below.</p>
        <div class="border-border overflow-x-auto rounded-lg border bg-[#050505]">
            <div class="min-w-max" inert>
                <div class="border-border bg-void/80 border-b px-4">
                    <HeaderLayoutBar bar="top" :zones="resolved.top" />
                </div>
                <div v-if="showNavRow" class="bg-void/60 px-4">
                    <HeaderLayoutBar bar="nav" :zones="resolved.nav" />
                </div>
                <div v-else class="text-dim px-4 py-2 text-[11px]">
                    The nav row is empty: it is not shown.
                </div>
            </div>
        </div>
    </div>
</template>
