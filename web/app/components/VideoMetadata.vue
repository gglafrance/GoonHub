<script setup lang="ts">
import type { Video } from '~/types/video';

defineProps<{
    video: Video;
}>();

const { formatDuration, formatSize, formatDate } = useTime();
</script>

<template>
    <div class="sticky top-28 space-y-3">
        <div class="border-border bg-surface/50 rounded-xl border p-4 backdrop-blur-sm">
            <h1 class="text-sm leading-snug font-semibold text-white">
                {{ video.title }}
            </h1>

            <div class="mt-4 space-y-0">
                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Duration</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatDuration(video.duration) }}
                    </span>
                </div>

                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Size</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatSize(video.size) }}
                    </span>
                </div>

                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Views</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ video.view_count }}
                    </span>
                </div>

                <div class="flex items-center justify-between py-2.5">
                    <span class="text-dim text-[11px]">Added</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatDate(video.created_at) }}
                    </span>
                </div>
            </div>

            <div class="border-border mt-4 border-t pt-3">
                <span class="text-dim text-[10px] font-medium tracking-wider uppercase">File</span>
                <p class="text-dim/70 mt-1 font-mono text-[10px] break-all">
                    {{ video.original_filename }}
                </p>
            </div>
        </div>

        <!-- Actions -->
        <div class="border-border bg-surface/50 rounded-xl border p-3 backdrop-blur-sm">
            <div class="flex gap-2">
                <button
                    class="border-border bg-panel text-dim hover:border-border-hover flex-1
                        rounded-lg border py-2 text-[11px] font-medium transition-all
                        hover:text-white"
                >
                    <Icon name="heroicons:share" size="12" class="mr-1" />
                    Share
                </button>
                <button
                    class="border-border bg-panel text-dim hover:border-lava/30 hover:text-lava
                        flex-1 rounded-lg border py-2 text-[11px] font-medium transition-all"
                >
                    <Icon name="heroicons:heart" size="12" class="mr-1" />
                    Favorite
                </button>
            </div>
        </div>
    </div>
</template>
