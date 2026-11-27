<script setup lang="ts">
import type { ProfileBlock, ProfileCustomListPayload } from '~/types/profile';

// custom_list: the first videos of one of the owner's custom lists, as a grid or a compact row
// (profile block). The frame shows the list title and the "Browse" link.
const props = defineProps<{ block: ProfileBlock<ProfileCustomListPayload>; owner: boolean }>();

const row = computed(() => props.block.widget?.display === 'row');
</script>

<template>
    <div v-if="row" class="scrollbar-none -mx-1 flex snap-x gap-3 overflow-x-auto px-1 pb-1">
        <div
            v-for="item in props.block.scenes ?? []"
            :key="item.scene.id"
            class="w-44 shrink-0 snap-start sm:w-52"
        >
            <SceneCard :scene="item.scene" fluid />
        </div>
    </div>
    <ProfileScenesGrid v-else :scenes="props.block.scenes ?? []" figure="none" />
</template>
