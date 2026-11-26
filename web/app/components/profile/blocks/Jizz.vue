<script setup lang="ts">
import type { ProfileBlock, ProfileJizzPayload } from '~/types/profile';

// jizz: the O total and the videos the owner came to last, with their O count (profile block).
const props = defineProps<{ block: ProfileBlock<ProfileJizzPayload>; owner: boolean }>();

const number = new Intl.NumberFormat();
</script>

<template>
    <div class="space-y-3">
        <div v-if="props.block.widget" class="text-dim text-[11px]">
            <span class="font-mono text-white">{{ number.format(props.block.widget.total) }}</span>
            O on
            <span class="font-mono text-white">{{ number.format(props.block.widget.scenes) }}</span>
            {{ props.block.widget.scenes === 1 ? 'video' : 'videos' }}
        </div>
        <ProfileScenesGrid :scenes="props.block.scenes ?? []" figure="jizz" />
    </div>
</template>
