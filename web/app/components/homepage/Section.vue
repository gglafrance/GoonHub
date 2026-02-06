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
    const currentScenes = props.data?.scenes;

    // Emit refresh request to parent
    emit('refresh', props.section.id);

    // Wait for data to actually change (with timeout fallback)
    await new Promise<void>((resolve) => {
        const timeout = setTimeout(() => {
            stopWatch();
            resolve();
        }, 5000); // 5s max wait

        const stopWatch = watch(
            () => props.data?.scenes,
            (newScenes) => {
                // Data changed or we got new data
                if (newScenes !== currentScenes) {
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
    const sort = props.section.sort;
    const seed = props.data?.seed;

    // Helper: append sort & seed params when section uses random sort
    const withSortParams = (base: string) => {
        if (sort === 'random' && seed) {
            const sep = base.includes('?') ? '&' : '?';
            return `${base}${sep}sort=random&seed=${seed}`;
        }
        if (sort && sort !== 'created_at_desc') {
            const sep = base.includes('?') ? '&' : '?';
            return `${base}${sep}sort=${sort}`;
        }
        return base;
    };

    switch (type) {
        case 'latest':
            return withSortParams('/search?sort=created_at_desc');
        case 'actor':
            return withSortParams(
                config.actor_name
                    ? `/search?actors=${encodeURIComponent(config.actor_name as string)}`
                    : '/search',
            );
        case 'studio':
            return withSortParams(
                config.studio_name
                    ? `/search?studio=${encodeURIComponent(config.studio_name as string)}`
                    : '/search',
            );
        case 'tag':
            return withSortParams(
                config.tag_name
                    ? `/search?tags=${encodeURIComponent(config.tag_name as string)}`
                    : '/search',
            );
        case 'saved_search': {
            if (!config.saved_search_uuid) return '/search';
            const params = new URLSearchParams({
                saved: config.saved_search_uuid as string,
            });
            if (seed) params.set('seed', String(seed));
            return `/search?${params}`;
        }
        case 'liked':
            return withSortParams('/search?liked=true');
        case 'most_viewed':
            return withSortParams('/search?sort=view_count_desc');
        case 'continue_watching':
            return '/history?filter=in_progress';
        case 'playlist':
            return '/playlists';
        default:
            return withSortParams('/search');
    }
});
</script>

<template>
    <div class="mb-6 sm:mb-8">
        <!-- Section Header -->
        <div class="mb-3 flex items-center justify-between sm:mb-4">
            <div class="flex min-w-0 items-center gap-2 sm:gap-3">
                <h2
                    class="truncate text-xs font-semibold tracking-wide text-white uppercase
                        sm:text-sm"
                >
                    {{ displayTitle }}
                </h2>
                <span
                    v-if="data"
                    class="border-border bg-panel text-dim shrink-0 rounded-full border px-1.5
                        py-0.5 font-mono text-[9px] sm:px-2 sm:text-[10px]"
                >
                    {{ data.total }}
                </span>
            </div>
            <div class="flex shrink-0 items-center gap-1 sm:gap-2">
                <button
                    :disabled="isRefreshing"
                    class="text-dim flex h-7 w-7 items-center justify-center rounded-md
                        transition-colors hover:bg-white/5 hover:text-white disabled:opacity-50
                        sm:h-auto sm:w-auto sm:rounded-none sm:bg-transparent
                        sm:hover:bg-transparent"
                    @click="handleRefresh"
                >
                    <Icon
                        name="heroicons:arrow-path"
                        size="16"
                        class="sm:h-3.5 sm:w-3.5"
                        :class="{ 'animate-spin': isRefreshing }"
                    />
                </button>
                <NuxtLink
                    :to="seeAllLink"
                    class="text-dim hover:text-lava flex items-center gap-0.5 rounded-md px-2 py-1.5
                        text-xs transition-colors hover:bg-white/5 sm:gap-1 sm:rounded-none
                        sm:bg-transparent sm:p-0 sm:hover:bg-transparent"
                >
                    <span class="hidden sm:inline">See all</span>
                    <span class="sm:hidden">All</span>
                    <Icon name="heroicons:chevron-right" size="14" />
                </NuxtLink>
            </div>
        </div>

        <!-- Content -->
        <div v-if="!data" class="flex h-48 items-center justify-center">
            <LoadingSpinner size="sm" />
        </div>

        <!-- Playlist section -->
        <template v-else-if="section.type === 'playlist'">
            <div
                v-if="!data.playlists || data.playlists.length === 0"
                class="border-border flex h-48 items-center justify-center rounded-lg border
                    border-dashed"
            >
                <p class="text-dim text-sm">No playlists in this section</p>
            </div>
            <HomepagePlaylistGrid v-else :playlists="data.playlists" />
        </template>

        <!-- Scene sections -->
        <template v-else>
            <div
                v-if="data.scenes.length === 0"
                class="border-border flex h-48 items-center justify-center rounded-lg border
                    border-dashed"
            >
                <p class="text-dim text-sm">No scenes in this section</p>
            </div>

            <HomepageSectionGrid
                v-else
                :scenes="data.scenes"
                :watch-progress="data.watch_progress"
                :ratings="data.ratings"
            />
        </template>
    </div>
</template>
