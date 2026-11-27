<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// because_you_watched widget: scenes related to a recently watched scene.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ source: { id: number; title: string } | null }>;
}>();

const source = computed(() => props.data.widget?.source ?? null);
</script>

<template>
    <div>
        <HomepageWidgetsScenesToolbar v-if="source">
            Because you watched
            <NuxtLink :to="`/watch/${source.id}`" class="hover:text-lava font-medium text-white">
                {{ source.title }}
            </NuxtLink>
        </HomepageWidgetsScenesToolbar>
        <HomepageWidgetsScenesEmpty
            v-if="data.scenes.length === 0"
            icon="heroicons:link"
            message="Watch a few scenes first: similar scenes will show up here."
        />
        <HomepageSectionGrid
            v-else
            :scenes="data.scenes"
            :watch-progress="data.watch_progress"
            :ratings="data.ratings"
        />
    </div>
</template>
