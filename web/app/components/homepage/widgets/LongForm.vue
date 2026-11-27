<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// long_form widget: long scenes, in progress first, then unwatched ones.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ min_duration: number; in_progress: number[] }>;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const { busy, request } = useSceneWidgetRefresh(
    () => props.data,
    () => emit('refresh'),
);

const minMinutes = computed(() => Math.round((props.data.widget?.min_duration ?? 1200) / 60));
const inProgress = computed(() => props.data.widget?.in_progress.length ?? 0);
</script>

<template>
    <div>
        <HomepageWidgetsScenesToolbar
            action-label="Shuffle"
            action-icon="heroicons:arrow-path-rounded-square"
            :busy="busy"
            @action="request"
        >
            Scenes of {{ minMinutes }} min or more
            <template v-if="inProgress">
                · <span class="text-lava">{{ inProgress }} in progress</span>
            </template>
        </HomepageWidgetsScenesToolbar>
        <HomepageWidgetsScenesEmpty
            v-if="data.scenes.length === 0"
            icon="heroicons:film"
            :message="`No long scene (${minMinutes} min or more) left to watch.`"
        />
        <HomepageSectionGrid
            v-else
            :scenes="data.scenes"
            :watch-progress="data.watch_progress"
            :ratings="data.ratings"
        />
    </div>
</template>
