<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// discover widget: random scenes never watched, reshuffled on demand.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ unwatched_total: number }>;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const { busy, request } = useSceneWidgetRefresh(
    () => props.data,
    () => emit('refresh'),
);

const unwatched = computed(() => props.data.widget?.unwatched_total ?? props.data.total);
</script>

<template>
    <div>
        <HomepageWidgetsScenesToolbar
            action-label="Shuffle"
            action-icon="heroicons:arrow-path-rounded-square"
            :busy="busy"
            @action="request"
        >
            <span class="font-mono text-white">{{ unwatched.toLocaleString() }}</span>
            scenes you have never watched
        </HomepageWidgetsScenesToolbar>
        <HomepageWidgetsScenesEmpty
            v-if="data.scenes.length === 0"
            icon="heroicons:sparkles"
            message="You have watched everything matching this section. Widen its filters."
        />
        <HomepageSectionGrid
            v-else
            :scenes="data.scenes"
            :watch-progress="data.watch_progress"
            :ratings="data.ratings"
        />
    </div>
</template>
