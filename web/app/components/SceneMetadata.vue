<script setup lang="ts">
import type { Scene } from '~/types/scene';

defineProps<{
    scene: Scene;
}>();

const { formatDuration, formatSize, formatBitRate, formatFrameRate } = useFormatter();

const formatResolution = (w?: number, h?: number): string => {
    if (!w || !h) return '';
    return `${w}x${h}`;
};

const formatCodec = (codec?: string): string => {
    if (!codec) return '';
    return codec.toUpperCase();
};

const getResolutionLabel = (h?: number): string => {
    if (!h) return '';
    if (h >= 4320) return '8K';
    if (h >= 2880) return '5K';
    if (h >= 2160) return '4K';
    if (h >= 1440) return 'QHD';
    if (h >= 1080) return 'FHD';
    if (h >= 720) return 'HD';
    return 'SD';
};
</script>

<template>
    <div class="sticky top-28 space-y-3">
        <!-- Info Section -->
        <div class="border-border bg-surface/30 rounded-xl border p-4 backdrop-blur-sm">
            <h1 class="text-sm leading-snug font-semibold text-white">
                {{ scene.title }}
            </h1>

            <div class="mt-4 space-y-0">
                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Duration</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatDuration(scene.duration) }}
                    </span>
                </div>

                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Size</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatSize(scene.size) }}
                    </span>
                </div>

                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Views</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ scene.view_count }}
                    </span>
                </div>

                <div class="flex items-center justify-between py-2.5">
                    <span class="text-dim text-[11px]">Added</span>
                    <span class="text-muted font-mono text-[11px]">
                        <NuxtTime
                            :datetime="scene.created_at"
                            year="numeric"
                            month="short"
                            day="numeric"
                        />
                    </span>
                </div>
            </div>
        </div>

        <!-- Technical Section -->
        <div
            v-if="scene.width || scene.frame_rate || scene.bit_rate || scene.video_codec"
            class="border-border bg-surface/30 rounded-xl border p-4 backdrop-blur-sm"
        >
            <span class="text-dim text-[10px] font-medium tracking-wider uppercase">Technical</span>
            <div class="mt-2 space-y-0">
                <div
                    v-if="scene.width && scene.height"
                    class="border-border flex items-center justify-between border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Resolution</span>
                    <span class="flex items-center gap-1.5">
                        <span class="text-muted font-mono text-[11px]">
                            {{ formatResolution(scene.width, scene.height) }}
                        </span>
                        <span
                            class="border-lava/30 bg-lava/10 text-lava rounded px-1 py-px text-[9px]
                                leading-tight font-bold"
                        >
                            {{ getResolutionLabel(scene.height) }}
                        </span>
                    </span>
                </div>

                <div
                    v-if="scene.frame_rate"
                    class="border-border flex items-center justify-between border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Frame Rate</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatFrameRate(scene.frame_rate) }}
                    </span>
                </div>

                <div
                    v-if="scene.bit_rate"
                    class="border-border flex items-center justify-between border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Bit Rate</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatBitRate(scene.bit_rate) }}
                    </span>
                </div>

                <div
                    v-if="scene.video_codec"
                    class="border-border flex items-center justify-between py-2.5"
                    :class="{ 'border-b': scene.audio_codec }"
                >
                    <span class="text-dim text-[11px]">Video Codec</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatCodec(scene.video_codec) }}
                    </span>
                </div>

                <div v-if="scene.audio_codec" class="flex items-center justify-between py-2.5">
                    <span class="text-dim text-[11px]">Audio Codec</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatCodec(scene.audio_codec) }}
                    </span>
                </div>
            </div>
        </div>

        <!-- File Section -->
        <div class="border-border bg-surface/30 rounded-xl border p-4 backdrop-blur-sm">
            <span class="text-dim text-[10px] font-medium tracking-wider uppercase">File</span>
            <div class="mt-2 space-y-0">
                <div class="border-border border-b py-2.5">
                    <span class="text-dim text-[11px]">Filename</span>
                    <p class="text-dim/70 mt-0.5 font-mono text-[10px] break-all">
                        {{ scene.original_filename }}
                    </p>
                </div>

                <div v-if="scene.stored_path" class="border-border border-b py-2.5">
                    <span class="text-dim text-[11px]">Path</span>
                    <p class="text-dim/70 mt-0.5 font-mono text-[10px] break-all">
                        {{ scene.stored_path }}
                    </p>
                </div>

                <div v-if="scene.file_created_at" class="flex items-center justify-between py-2.5">
                    <span class="text-dim text-[11px]">File Date</span>
                    <span class="text-muted font-mono text-[11px]">
                        <NuxtTime
                            :datetime="scene.file_created_at"
                            year="numeric"
                            month="short"
                            day="numeric"
                        />
                    </span>
                </div>
            </div>
        </div>

        <!-- Actions -->
        <div class="border-border bg-surface/30 rounded-xl border p-3 backdrop-blur-sm">
            <div class="flex gap-2">
                <button
                    class="border-border bg-panel text-dim hover:border-border-hover flex-1
                        rounded-lg border py-2 text-[11px] font-medium transition-all
                        hover:text-white"
                >
                    <Icon name="heroicons:share" size="12" class="mr-1" />
                    Share
                </button>
            </div>
        </div>
    </div>
</template>
