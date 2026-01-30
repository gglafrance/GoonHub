<script setup lang="ts">
import type { HomepageSection, HomepageSectionData, SectionType } from '~/types/homepage';
import { SECTION_TYPE_LABELS } from '~/types/homepage';

const props = defineProps<{
    section: HomepageSection;
    data?: HomepageSectionData;
}>();

const emit = defineEmits<{
    refresh: [sectionId: string];
}>();

const isRefreshing = ref(false);

const handleRefresh = async () => {
    if (isRefreshing.value) return;
    isRefreshing.value = true;

    // Capture current data state to detect when refresh completes
    const currentVideos = props.data?.videos;

    // Emit refresh request to parent
    emit('refresh', props.section.id);

    // Wait for data to actually change (with timeout fallback)
    await new Promise<void>((resolve) => {
        const timeout = setTimeout(() => {
            stopWatch();
            resolve();
        }, 5000); // 5s max wait

        const stopWatch = watch(
            () => props.data?.videos,
            (newVideos) => {
                // Data changed or we got new data
                if (newVideos !== currentVideos) {
                    clearTimeout(timeout);
                    stopWatch();
                    resolve();
                }
            },
            { immediate: false },
        );
    });

    isRefreshing.value = false;
};

// Generate descriptive title with type prefix and specific name
const displayTitle = computed(() => {
    const type = props.section.type as SectionType;
    const config = props.section.config;
    const typeLabel = SECTION_TYPE_LABELS[type];

    // If user has a custom title, use it
    if (props.section.title && props.section.title !== typeLabel) {
        return props.section.title;
    }

    // Otherwise, build a descriptive title
    switch (type) {
        case 'actor':
            return config.actor_name ? `Actor \u2022 ${config.actor_name}` : 'Actor';
        case 'studio':
            return config.studio_name ? `Studio \u2022 ${config.studio_name}` : 'Studio';
        case 'tag':
            return config.tag_name ? `Tag \u2022 ${config.tag_name}` : 'Tag';
        case 'saved_search':
            return config.saved_search_name
                ? `Search \u2022 ${config.saved_search_name}`
                : 'Saved Search';
        default:
            return typeLabel;
    }
});

const seeAllLink = computed(() => {
    const type = props.section.type as SectionType;
    const config = props.section.config;

    switch (type) {
        case 'latest':
            return '/search?sort=created_at_desc';
        case 'actor':
            return config.actor_name
                ? `/search?actors=${encodeURIComponent(config.actor_name as string)}`
                : '/search';
        case 'studio':
            return config.studio_name
                ? `/search?studio=${encodeURIComponent(config.studio_name as string)}`
                : '/search';
        case 'tag':
            return config.tag_name
                ? `/search?tags=${encodeURIComponent(config.tag_name as string)}`
                : '/search';
        case 'saved_search':
            return config.saved_search_uuid
                ? `/search?saved=${config.saved_search_uuid}`
                : '/search';
        case 'liked':
            return '/search?liked=true';
        case 'most_viewed':
            return '/search?sort=view_count_desc';
        case 'continue_watching':
            return '/history?filter=in_progress';
        default:
            return '/search';
    }
});
</script>

<template>
    <div class="mb-8">
        <!-- Section Header -->
        <div class="mb-4 flex items-center justify-between">
            <div class="flex items-center gap-3">
                <h2 class="text-sm font-semibold tracking-wide text-white uppercase">
                    {{ displayTitle }}
                </h2>
                <span
                    v-if="data"
                    class="border-border bg-panel text-dim rounded-full border px-2 py-0.5 font-mono
                        text-[10px]"
                >
                    {{ data.total }}
                </span>
            </div>
            <div class="flex items-center gap-2">
                <button
                    @click="handleRefresh"
                    :disabled="isRefreshing"
                    class="text-dim flex items-center gap-1 text-xs transition-colors
                        hover:text-white disabled:opacity-50"
                >
                    <Icon
                        name="heroicons:arrow-path"
                        size="14"
                        :class="{ 'animate-spin': isRefreshing }"
                    />
                </button>
                <NuxtLink
                    :to="seeAllLink"
                    class="text-dim hover:text-lava flex items-center gap-1 text-xs
                        transition-colors"
                >
                    See all
                    <Icon name="heroicons:chevron-right" size="14" />
                </NuxtLink>
            </div>
        </div>

        <!-- Content -->
        <div v-if="!data" class="flex h-48 items-center justify-center">
            <LoadingSpinner size="sm" />
        </div>

        <div
            v-else-if="data.videos.length === 0"
            class="border-border flex h-48 items-center justify-center rounded-lg border
                border-dashed"
        >
            <p class="text-dim text-sm">No videos in this section</p>
        </div>

        <HomepageSectionGrid
            v-else
            :videos="data.videos"
            :watch-progress="data.watch_progress"
            :ratings="data.ratings"
        />
    </div>
</template>
