<script setup lang="ts">
import type { Video } from '~/types/video';

const props = defineProps<{
    video: Video;
}>();

const { formatDuration } = useTime();
const { frames, loadFrames, getFrameByTimestamp, hasError } =
    useFramePreloader(props.video);

const currentThumbnail = ref<string | null>(null);
const isHoveringCard = ref(false);
const previewTimestamp = ref(0);
const thumbnailContainer = ref<HTMLElement>();

const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    });
};

const isProcessing = computed(() => {
    return (
        props.video.processing_status === 'pending' ||
        props.video.processing_status === 'processing'
    );
});

const showPreviewBar = computed(() => {
    return !isProcessing.value && props.video.frame_count && props.video.frame_count > 0;
});

watch(isHoveringCard, (hovering) => {
    if (hovering && showPreviewBar.value) {
        loadFrames();
    } else if (!hovering) {
        previewTimestamp.value = 0;
        currentThumbnail.value = props.video.thumbnail_path
            ? `/thumbnails/${props.video.id}`
            : null;
    }
});

const handleMouseMove = (e: MouseEvent) => {
    if (!thumbnailContainer.value || props.video.duration === 0) return;

    const rect = thumbnailContainer.value.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const percent = Math.max(0, Math.min(1, x / rect.width));

    previewTimestamp.value = percent * props.video.duration;

    const frame = getFrameByTimestamp(previewTimestamp.value);
    if (frame) {
        currentThumbnail.value = frame.src;
    }
};

const handleMouseEnter = () => {
    isHoveringCard.value = true;
    previewTimestamp.value = props.video.duration / 2;

    if (showPreviewBar.value) {
        const frame = getFrameByTimestamp(previewTimestamp.value);
        if (frame) {
            currentThumbnail.value = frame.src;
        }
    }
};

const handleMouseLeave = () => {
    isHoveringCard.value = false;
};

onMounted(() => {
    if (props.video.thumbnail_path) {
        currentThumbnail.value = `/thumbnails/${props.video.id}`;
    }
});
</script>

<template>
    <div
        class="group bg-secondary/50 hover:bg-secondary hover:shadow-neon-green/10
            hover:border-neon-green/50 relative overflow-hidden rounded-2xl border border-white/5
            backdrop-blur-md transition-all duration-300 hover:shadow-lg"
        @mouseenter="isHoveringCard = true"
        @mouseleave="isHoveringCard = false"
    >
        <div
            ref="thumbnailContainer"
            class="relative aspect-video w-full cursor-pointer bg-black/50"
            @mouseenter="handleMouseEnter"
            @mouseleave="handleMouseLeave"
            @mousemove="handleMouseMove"
        >
            <img
                v-if="currentThumbnail"
                :src="currentThumbnail"
                class="absolute inset-0 h-full w-full object-cover transition-none"
                :alt="video.title"
            />

            <div v-else-if="isProcessing" class="absolute inset-0 flex items-center justify-center">
                <Icon name="heroicons:arrow-path" size="48" class="animate-spin text-gray-600" />
            </div>

            <div
                v-else-if="frames.length > 0 && hasError(frames.length - 1)"
                class="absolute inset-0 flex items-center justify-center"
            >
                <Icon name="heroicons:exclamation-triangle" size="48" class="text-neon-red" />
            </div>

            <div
                v-else
                class="group-hover:text-neon-green absolute inset-0 flex items-center justify-center
                    text-gray-600 transition-colors"
            >
                <Icon name="heroicons:play" size="48" />
            </div>

            <div
                v-if="video.duration > 0"
                class="absolute right-2 bottom-2 rounded bg-black/80 px-1.5 py-0.5 text-xs
                    font-medium text-white backdrop-blur-sm"
            >
                {{ formatDuration(video.duration) }}
            </div>

            <VideoPreviewBar
                v-if="isHoveringCard && showPreviewBar"
                :video="video"
                :preview-timestamp="previewTimestamp"
            />
        </div>

        <div class="p-4">
            <h3
                class="group-hover:text-neon-green truncate text-lg font-bold text-white
                    transition-colors"
                :title="video.title"
            >
                {{ video.title }}
            </h3>
            <div class="mt-2 flex items-center justify-between text-xs text-gray-400">
                <span>{{ formatSize(video.size) }}</span>
                <span>{{ formatDate(video.created_at) }}</span>
            </div>
        </div>
    </div>
</template>
