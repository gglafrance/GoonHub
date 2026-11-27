<script setup lang="ts">
import type { Component } from 'vue';
import type { ProfileBlock, ProfileCustomListPayload } from '~/types/profile';

// Renders a profile block with components/profile/blocks/<PascalType>.vue inside its frame.
// Block component contract: props { block, owner }. Locked blocks (the owner's "private"
// placeholder) and failed ones have no data and are rendered by the frame alone.
const props = defineProps<{ block: ProfileBlock; owner: boolean }>();

const blockModules = import.meta.glob('./blocks/*.vue');

// "See all": the full list a block opens (liked, rated, jizz videos), same privacy item
const route = useRoute();
const seeAll = computed(() => {
    if (props.block.locked || props.block.failed || props.block.empty) return null;
    const username = String(route.params.username ?? '');
    if (props.block.type === 'custom_list') {
        const id = (props.block.widget as ProfileCustomListPayload | undefined)?.list_id;
        return id
            ? { to: `/u/${encodeURIComponent(username)}/lists/${id}`, label: 'Browse' }
            : null;
    }
    const list = PROFILE_BLOCK_LISTS[props.block.type];
    return list ? { to: profileTabPath(username, list), label: 'See all' } : null;
});

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
            <NuxtLink
                v-if="seeAll"
                :to="seeAll.to"
                class="text-dim hover:text-lava ml-auto flex items-center gap-0.5 text-[11px]
                    transition-colors"
            >
                {{ seeAll.label }}
                <Icon name="heroicons:chevron-right" size="12" />
            </NuxtLink>
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
