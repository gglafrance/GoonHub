<script setup lang="ts">
import type { Video } from '~/types/video';

const props = defineProps<{
    video: Video;
}>();

const explorerStore = useExplorerStore();
const { formatDuration, formatSize } = useFormatter();

const isSelected = computed(() => explorerStore.isVideoSelected(props.video.id));

const isProcessing = computed(() => isVideoProcessing(props.video));

const thumbnailUrl = computed(() => {
    if (!props.video.thumbnail_path) return null;
    const base = `/thumbnails/${props.video.id}`;
    const v = props.video.updated_at ? new Date(props.video.updated_at).getTime() : '';
    return v ? `${base}?v=${v}` : base;
});

const handleCheckboxClick = (event: Event) => {
    event.preventDefault();
    event.stopPropagation();
    explorerStore.toggleVideoSelection(props.video.id);
};

const handleCardClick = (event: MouseEvent) => {
    // If shift or ctrl is held, toggle selection instead of navigating
    if (event.shiftKey || event.ctrlKey || event.metaKey) {
        event.preventDefault();
        explorerStore.toggleVideoSelection(props.video.id);
    }
};
</script>

<template>
    <div class="group relative">
        <!-- Selection Checkbox -->
        <button
            @click="handleCheckboxClick"
            class="absolute top-2 left-2 z-10 flex h-5 w-5 items-center justify-center rounded
                border transition-all"
            :class="
                isSelected
                    ? 'bg-lava border-lava text-white'
                    : 'border-white/30 bg-void/60 text-transparent hover:border-white/50 group-hover:text-white/50'
            "
        >
            <Icon name="heroicons:check" size="12" />
        </button>

        <!-- Video Link -->
        <NuxtLink
            :to="`/watch/${video.id}`"
            @click="handleCardClick"
            class="border-border bg-surface hover:border-border-hover hover:bg-elevated block
                overflow-hidden rounded-lg border transition-all duration-200"
            :class="isSelected ? 'ring-lava/50 ring-2' : ''"
        >
            <div class="bg-void relative aspect-video">
                <img
                    v-if="thumbnailUrl"
                    :src="thumbnailUrl"
                    class="absolute inset-0 h-full w-full object-cover transition-transform
                        duration-300 group-hover:scale-[1.03]"
                    :alt="video.title"
                    loading="lazy"
                />

                <div
                    v-else-if="isProcessing"
                    class="absolute inset-0 flex items-center justify-center"
                >
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

                <!-- Selected overlay -->
                <div
                    v-if="isSelected"
                    class="bg-lava/10 absolute inset-0 pointer-events-none"
                ></div>

                <!-- Hover overlay -->
                <div
                    class="bg-lava/0 group-hover:bg-lava/5 absolute inset-0 transition-colors
                        duration-200 pointer-events-none"
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
    </div>
</template>
