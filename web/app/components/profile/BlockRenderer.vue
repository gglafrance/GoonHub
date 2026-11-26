<script setup lang="ts">
import type { Component } from 'vue';
import type { ProfileBlock } from '~/types/profile';

// Renders a profile block with components/profile/blocks/<PascalType>.vue inside its frame.
// Block component contract: props { block, owner }. Locked blocks (the owner's "private"
// placeholder) and failed ones have no data and are rendered by the frame alone.
const props = defineProps<{ block: ProfileBlock; owner: boolean }>();

const blockModules = import.meta.glob('./blocks/*.vue');

const pascal = (type: string) =>
    type
        .split('_')
        .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
        .join('');

const component = computed<Component | null>(() => {
    const loader = blockModules[`./blocks/${pascal(props.block.type)}.vue`];
    return loader ? defineAsyncComponent(loader as () => Promise<Component>) : null;
});
</script>

<template>
    <section class="glass-panel p-4">
        <div class="mb-3 flex items-center gap-2">
            <Icon
                :name="PROFILE_BLOCK_ICONS[block.type] ?? 'heroicons:squares-2x2'"
                size="14"
                class="text-lava"
            />
            <h2 class="text-xs font-semibold tracking-wide text-white uppercase">
                {{ block.label }}
            </h2>
            <span v-if="block.total !== undefined && !block.locked" class="text-dim text-[11px]">
                {{ block.total }}
            </span>
        </div>
        <div
            v-if="block.locked"
            class="border-border text-dim flex items-center justify-center gap-1.5 rounded-lg border
                border-dashed px-3 py-6 text-xs"
        >
            <Icon name="heroicons:lock-closed" size="13" />
            Private
        </div>
        <div
            v-else-if="block.failed"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            This block could not be loaded.
        </div>
        <div
            v-else-if="block.empty"
            class="border-border text-dim rounded-lg border border-dashed px-3 py-6 text-center
                text-xs"
        >
            {{ owner ? 'Nothing to show yet.' : 'Nothing here yet.' }}
        </div>
        <component :is="component" v-else-if="component" :block="block" :owner="owner" />
    </section>
</template>
