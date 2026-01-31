<script setup lang="ts">
useHead({ title: 'Explorer' });

useSeoMeta({
    title: 'Explorer',
    ogTitle: 'Explorer - GoonHub',
    description: 'Browse your video storage paths',
    ogDescription: 'Browse your video storage paths',
});

const explorerStore = useExplorerStore();

onMounted(async () => {
    // Reset to root view
    explorerStore.currentStoragePathID = null;
    explorerStore.currentPath = '';
    explorerStore.subfolders = [];
    explorerStore.videos = [];
    explorerStore.totalVideos = 0;
    explorerStore.clearSelection();

    await explorerStore.loadStoragePaths();
});

// Don't reset on unmount - let the destination page handle state

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Header -->
            <div class="mb-6">
                <div class="flex items-center justify-between">
                    <h1 class="text-lg font-semibold text-white">Explorer</h1>
                </div>
            </div>

            <!-- Error -->
            <ErrorAlert v-if="explorerStore.error" :message="explorerStore.error" class="mb-4" />

            <!-- Loading State -->
            <div
                v-if="explorerStore.isLoading && explorerStore.storagePaths.length === 0"
                class="flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading..." />
            </div>

            <!-- Storage Path List -->
            <ExplorerStoragePathList v-else />
        </div>
    </div>
</template>
