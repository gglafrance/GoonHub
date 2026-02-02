<script setup lang="ts">
import type { MarkerWithScene } from '~/types/marker';

const route = useRoute();
const router = useRouter();
const { fetchMarkersByLabel } = useApiMarkers();
const { formatDuration } = useFormatter();
const settingsStore = useSettingsStore();

const markers = ref<MarkerWithScene[]>([]);
const total = ref(0);
const currentPage = ref(1);
const limit = computed(() => settingsStore.videosPerPage);
const isLoading = ref(true);
const error = ref<string | null>(null);

const label = computed(() => decodeURIComponent(route.params.label as string));

const pageTitle = computed(() => label.value || 'Markers');
useHead({ title: pageTitle });

// Dynamic OG metadata
watch(
    [label, total],
    ([l, t]) => {
        if (l) {
            useSeoMeta({
                title: l,
                ogTitle: `${l} - Markers`,
                description: `${l} - ${t} markers on GoonHub`,
                ogDescription: `${l} - ${t} markers on GoonHub`,
            });
        }
    },
    { immediate: true },
);

const loadMarkers = async (page = 1) => {
    isLoading.value = true;
    error.value = null;
    try {
        const response = await fetchMarkersByLabel(label.value, page, limit.value);
        markers.value = response.data;
        total.value = response.pagination.total_items;
        currentPage.value = page;
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load markers';
    } finally {
        isLoading.value = false;
    }
};

onMounted(() => {
    loadMarkers();
});

watch(
    () => route.params.label,
    () => {
        loadMarkers();
    },
);

watch(
    () => currentPage.value,
    (newPage) => {
        loadMarkers(newPage);
    },
);

const goBack = () => {
    router.push('/markers');
};

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Back Button -->
            <button
                @click="goBack"
                class="text-dim hover:text-lava mb-4 flex items-center gap-1 text-sm
                    transition-colors"
            >
                <Icon name="heroicons:arrow-left" size="16" />
                Back to Markers
            </button>

            <!-- Header -->
            <div class="mb-4">
                <div class="flex items-center justify-between">
                    <h1 class="text-lg font-semibold text-white">{{ label }}</h1>
                    <span
                        class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                            font-mono text-[11px]"
                    >
                        {{ total }} markers
                    </span>
                </div>
            </div>

            <!-- Label Tag Manager -->
            <MarkersLabelTagManager :label="label" class="mb-6" />

            <!-- Error -->
            <ErrorAlert v-if="error" :message="error" class="mb-4" />

            <!-- Loading State -->
            <div
                v-if="isLoading && markers.length === 0"
                class="flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading markers..." />
            </div>

            <!-- Empty State -->
            <div
                v-else-if="markers.length === 0"
                class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                    border border-dashed text-center"
            >
                <div
                    class="bg-panel border-border flex h-10 w-10 items-center justify-center
                        rounded-lg border"
                >
                    <Icon name="heroicons:bookmark" size="20" class="text-dim" />
                </div>
                <p class="text-muted mt-3 text-sm">No markers found</p>
            </div>

            <!-- Markers Grid -->
            <div v-else>
                <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                    <NuxtLink
                        v-for="marker in markers"
                        :key="marker.id"
                        :to="`/watch/${marker.scene_id}?t=${marker.timestamp}`"
                        class="group relative block max-w-[320px] overflow-hidden rounded-lg
                            transition-transform duration-200"
                    >
                        <!-- Thumbnail -->
                        <div class="relative aspect-video w-full overflow-hidden bg-black/40">
                            <img
                                :src="`/marker-thumbnails/${marker.id}`"
                                :alt="marker.scene_title"
                                class="h-full w-full object-cover transition-transform duration-300
                                    group-hover:scale-105"
                                loading="lazy"
                            />

                            <!-- Gradient overlay -->
                            <div
                                class="pointer-events-none absolute inset-0 bg-linear-to-t
                                    from-black/80 via-black/20 to-transparent"
                            />

                            <!-- Timestamp badge -->
                            <div
                                class="absolute right-1.5 bottom-1.5 rounded bg-black/80 px-1.5
                                    py-0.5 text-[10px] font-semibold text-white/90 tabular-nums
                                    backdrop-blur-sm"
                            >
                                {{ formatDuration(marker.timestamp) }}
                            </div>
                        </div>

                        <!-- Info -->
                        <div class="border-border bg-surface border-t px-2 py-1.5">
                            <p
                                class="truncate text-xs font-medium text-white
                                    group-hover:text-white"
                            >
                                {{ marker.scene_title }}
                            </p>
                            <NuxtTime
                                :datetime="marker.created_at"
                                class="text-dim mt-0.5 text-[10px]"
                                relative
                            />
                        </div>
                    </NuxtLink>
                </div>

                <Pagination v-model="currentPage" :total="total" :limit="limit" />
            </div>
        </div>
    </div>
</template>
