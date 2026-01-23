<script setup lang="ts">
import type { Video } from '~/types/video';

const props = defineProps<{
    video: Video;
}>();

const { formatDuration, formatSize } = useFormatter();

const isProcessing = computed(() => isVideoProcessing(props.video));

const thumbnailUrl = computed(() => {
    return props.video.thumbnail_path ? `/thumbnails/${props.video.id}` : null;
});
</script>

<template>
    <NuxtLink
        :to="`/watch/${video.id}`"
        class="group border-border bg-surface hover:border-border-hover hover:bg-elevated relative
            block overflow-hidden rounded-lg border transition-all duration-200"
    >
        <div class="bg-void relative aspect-video w-full">
            <img
                v-if="thumbnailUrl"
                :src="thumbnailUrl"
                class="absolute inset-0 h-full w-full object-contain transition-transform duration-300
                    group-hover:scale-[1.03]"
                :alt="video.title"
                loading="lazy"
            />

            <div v-else-if="isProcessing" class="absolute inset-0 flex items-center justify-center">
                <LoadingSpinner size="sm" />
            </div>

            <div
                v-else
                class="text-dim group-hover:text-lava absolute inset-0 flex items-center
                    justify-center transition-colors"
            >
                <Icon name="heroicons:play" size="32" />
            </div>

            <!-- Duration badge -->
            <div
                v-if="video.duration > 0"
                class="bg-void/90 absolute right-1.5 bottom-1.5 rounded px-1.5 py-0.5 font-mono
                    text-[10px] font-medium text-white backdrop-blur-sm"
            >
                {{ formatDuration(video.duration) }}
            </div>

            <!-- Hover overlay -->
            <div
                class="bg-lava/0 group-hover:bg-lava/5 absolute inset-0 transition-colors
                    duration-200"
            ></div>
        </div>

        <div class="p-3">
            <h3
                class="truncate text-xs font-medium text-white/90 transition-colors
                    group-hover:text-white"
                :title="video.title"
            >
                {{ video.title }}
            </h3>
            <div class="text-dim mt-1.5 flex items-center justify-between font-mono text-[10px]">
                <span>{{ formatSize(video.size) }}</span>
                <NuxtTime :datetime="video.created_at" format="short" />
            </div>
        </div>
    </NuxtLink>
</template>
