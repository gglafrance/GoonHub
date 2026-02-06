<script setup lang="ts">
import type { MarkerLabelGroup } from '~/types/marker';

const props = defineProps<{
    group: MarkerLabelGroup;
    markerThumbnailType?: string;
}>();

const settingsStore = useSettingsStore();
const cyclingEnabled = computed(() => settingsStore.markerThumbnailCycling);
const isAnimated = computed(() => props.markerThumbnailType === 'animated');
const { observe } = useAnimatedMarkerPreview();

const ids = computed(() => {
    const arr = props.group.thumbnail_marker_ids;
    if (arr && arr.length > 0) return arr;
    return [props.group.thumbnail_marker_id];
});

const currentIndex = ref(0);
const showNext = ref(false);
const nextLoaded = ref(false);
let cycleTimer: ReturnType<typeof setTimeout> | null = null;

const currentUrl = computed(() => `/marker-thumbnails/${ids.value[currentIndex.value]}`);
const nextIndex = computed(() => (currentIndex.value + 1) % ids.value.length);
const nextUrl = computed(() => `/marker-thumbnails/${ids.value[nextIndex.value]}`);

// For animated mode, pick a random marker ID for the video
const animatedMarkerId = computed(() => {
    const arr = ids.value;
    return arr[Math.floor(Math.random() * arr.length)];
});
const animatedUrl = computed(() => `/marker-thumbnails/${animatedMarkerId.value}/animated`);

function stopCycling() {
    if (cycleTimer) {
        clearTimeout(cycleTimer);
        cycleTimer = null;
    }
}

function scheduleCycle() {
    stopCycling();
    if (!cyclingEnabled.value || ids.value.length <= 1) return;
    const delay = 2000 + Math.random() * 2000; // 2-4s
    cycleTimer = setTimeout(() => {
        // Preload next image then crossfade
        nextLoaded.value = false;
        const img = new Image();
        img.onload = () => {
            nextLoaded.value = true;
            showNext.value = true;
            setTimeout(() => {
                currentIndex.value = nextIndex.value;
                showNext.value = false;
                scheduleCycle();
            }, 400); // match CSS transition duration
        };
        img.onerror = () => {
            // Skip this thumbnail, advance index
            currentIndex.value = nextIndex.value;
            scheduleCycle();
        };
        img.src = nextUrl.value;
    }, delay);
}

watch(cyclingEnabled, (enabled) => {
    if (enabled) {
        scheduleCycle();
    } else {
        stopCycling();
    }
});

onMounted(() => {
    // Start at a random index so cards don't all show the same frame
    if (ids.value.length > 1) {
        currentIndex.value = Math.floor(Math.random() * ids.value.length);
    }
    scheduleCycle();
});

onBeforeUnmount(() => {
    stopCycling();
});
</script>

<template>
    <NuxtLink
        :to="`/markers/${encodeURIComponent(group.label)}`"
        class="group relative block max-w-[320px] overflow-hidden rounded-lg"
    >
        <!-- Thumbnail -->
        <div class="relative aspect-video w-full overflow-hidden bg-black/40">
            <!-- Animated video mode -->
            <video
                v-if="isAnimated"
                :ref="(el) => observe(el as HTMLVideoElement)"
                :src="animatedUrl"
                muted
                loop
                playsinline
                preload="none"
                class="absolute inset-0 h-full w-full object-contain transition-transform
                    duration-300 group-hover:scale-105"
            />
            <!-- Static image mode -->
            <template v-else>
                <img
                    :src="currentUrl"
                    :alt="group.label"
                    class="absolute inset-0 h-full w-full object-cover transition-transform
                        duration-300 group-hover:scale-105"
                    loading="lazy"
                />
                <!-- Next image for crossfade -->
                <img
                    v-if="showNext && nextLoaded"
                    :src="nextUrl"
                    :alt="group.label"
                    class="absolute inset-0 h-full w-full object-cover transition-opacity
                        duration-400 group-hover:scale-105"
                    :class="showNext ? 'opacity-100' : 'opacity-0'"
                />
            </template>

            <!-- Gradient overlay -->
            <div
                class="pointer-events-none absolute inset-0 bg-linear-to-t from-black/80
                    via-black/20 to-transparent"
            />

            <!-- Count badge -->
            <div
                class="absolute top-2 right-2 rounded-md bg-black/70 px-1.5 py-0.5 text-[10px]
                    font-semibold text-white/90 backdrop-blur-sm"
            >
                {{ group.count }} {{ group.count === 1 ? 'marker' : 'markers' }}
            </div>
        </div>

        <!-- Label text -->
        <div class="border-border bg-surface border-t px-2.5 py-2">
            <p class="truncate text-sm font-medium text-white group-hover:text-white">
                {{ group.label }}
            </p>
        </div>
    </NuxtLink>
</template>
