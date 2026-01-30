<script setup lang="ts">
import type { FolderInfo } from '~/types/explorer';

const props = defineProps<{
    folder: FolderInfo;
}>();

const router = useRouter();
const explorerStore = useExplorerStore();

const handleClick = () => {
    if (!explorerStore.currentStoragePathID) return;
    const cleanPath = props.folder.path.replace(/^\/+/, '');
    router.push(`/explorer/${explorerStore.currentStoragePathID}/${cleanPath}`);
};

const formatDuration = (seconds: number): string => {
    if (seconds <= 0) return '';
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (hours > 0) {
        return `${hours}h ${minutes}m`;
    }
    return `${minutes}m`;
};

const formatSize = (bytes: number): string => {
    if (bytes <= 0) return '';
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1) {
        return `${gb.toFixed(1)} GB`;
    }
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(0)} MB`;
};
</script>

<template>
    <button
        @click="handleClick"
        class="border-border bg-panel hover:border-lava/30 group flex flex-col items-center gap-2
            rounded-lg border p-3 text-center transition-all"
    >
        <div
            class="bg-lava/10 group-hover:bg-lava/20 flex h-10 w-10 items-center justify-center
                rounded-lg transition-colors"
        >
            <Icon name="heroicons:folder" size="20" class="text-lava" />
        </div>

        <div class="w-full min-w-0">
            <h4 class="truncate text-xs font-medium text-white">{{ folder.name }}</h4>
            <p class="text-dim mt-0.5 text-[10px]">
                {{ folder.video_count }} videos
                <template v-if="folder.total_duration > 0">
                    <span class="mx-0.5 opacity-50">·</span>
                    {{ formatDuration(folder.total_duration) }}
                </template>
                <template v-if="folder.total_size > 0">
                    <span class="mx-0.5 opacity-50">·</span>
                    {{ formatSize(folder.total_size) }}
                </template>
            </p>
        </div>
    </button>
</template>
