<script setup lang="ts">
import type { PlaybackNotice } from '~/types/scene';

// Overlay of the scene player when the scene can't be played (format not supported by the
// browser and no on-the-fly conversion, or conversion unavailable).
const props = defineProps<{
    notice: PlaybackNotice;
    downloadUrl?: string;
}>();

const emit = defineEmits<{
    retry: [];
}>();

const content = computed(() => {
    switch (props.notice.kind) {
        case 'disabled':
            return {
                icon: 'heroicons:film',
                title: "This format can't be played in the browser",
                text: 'On-the-fly transcoding is turned off.',
                retry: false,
            };
        case 'limit':
            return {
                icon: 'heroicons:clock',
                title: 'The conversion is busy',
                text: 'All on-the-fly conversion slots are in use. Try again in a moment.',
                retry: true,
            };
        default:
            return {
                icon: 'heroicons:exclamation-triangle',
                title: 'This scene could not be converted',
                text: 'The on-the-fly conversion of this file failed.',
                retry: true,
            };
    }
});
</script>

<template>
    <div
        class="absolute inset-0 z-20 flex items-center justify-center bg-black/75 p-4
            backdrop-blur-sm"
    >
        <div class="max-w-sm text-center">
            <div
                class="bg-lava/10 border-lava/20 mx-auto mb-3 flex h-12 w-12 items-center
                    justify-center rounded-full border"
            >
                <Icon :name="content.icon" size="22" class="text-lava" />
            </div>
            <h3 class="mb-1 text-sm font-semibold text-white">{{ content.title }}</h3>
            <p class="text-muted text-xs">{{ content.text }}</p>
            <p v-if="notice.reason" class="text-dim mt-1 text-[11px]">{{ notice.reason }}</p>
            <div class="mt-4 flex items-center justify-center gap-2">
                <button
                    v-if="content.retry"
                    type="button"
                    class="bg-lava hover:bg-lava-glow inline-flex items-center gap-1.5 rounded-lg
                        px-3 py-1.5 text-xs font-semibold text-white transition-all"
                    @click="emit('retry')"
                >
                    <Icon name="heroicons:arrow-path" size="14" />
                    Try again
                </button>
                <a
                    v-if="downloadUrl"
                    :href="downloadUrl"
                    download
                    class="border-border hover:border-border-hover inline-flex items-center gap-1.5
                        rounded-lg border px-3 py-1.5 text-xs font-medium text-white/80
                        transition-all hover:text-white"
                >
                    <Icon name="heroicons:arrow-down-tray" size="14" />
                    Download the original
                </a>
            </div>
        </div>
    </div>
</template>
