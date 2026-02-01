<script setup lang="ts">
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

// Inject centralized watch page data
const watchPageData = inject<WatchPageData>(WATCH_PAGE_DATA_KEY);

// Use centralized data for related scenes and loading state
const relatedScenes = computed(() => watchPageData?.relatedScenes.value ?? []);
const isLoading = computed(() => watchPageData?.loading.related ?? false);
</script>

<template>
    <div v-if="relatedScenes.length > 0 || isLoading" class="mt-6">
        <!-- Section Header -->
        <div class="mb-4 flex items-center justify-between">
            <div class="flex items-center gap-2">
                <div
                    class="from-lava/20 to-lava/5 flex h-7 w-7 items-center justify-center
                        rounded-lg bg-linear-to-br"
                >
                    <Icon name="heroicons:sparkles" size="14" class="text-lava" />
                </div>
                <h2 class="text-sm font-semibold text-white">Related Scenes</h2>
            </div>
            <span v-if="relatedScenes.length > 0" class="text-dim text-xs">
                {{ relatedScenes.length }} scenes
            </span>
        </div>

        <!-- Loading State -->
        <div
            v-if="isLoading"
            class="border-border bg-surface/50 flex h-48 items-center justify-center rounded-xl
                border"
        >
            <LoadingSpinner />
        </div>

        <!-- Related Scenes Grid -->
        <template v-else>
            <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                <SceneCard v-for="s in relatedScenes" :key="s.id" :scene="s" fluid />
            </div>
        </template>
    </div>
</template>
