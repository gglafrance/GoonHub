<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// showcase_4k widget: the highest resolution scenes.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ min_height: number; total: number }>;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const { busy, request } = useSceneWidgetRefresh(
    () => props.data,
    () => emit('refresh'),
);

const minHeight = computed(() => props.data.widget?.min_height ?? 2160);
const resolutionLabel = computed(() => (minHeight.value >= 2160 ? '4K' : `${minHeight.value}p`));
const isRandom = computed(() => (props.section.config?.order ?? 'random') !== 'newest');
</script>

<template>
    <div>
        <HomepageWidgetsScenesToolbar
            :action-label="isRandom ? 'Shuffle' : undefined"
            action-icon="heroicons:arrow-path-rounded-square"
            :busy="busy"
            @action="request"
        >
            <span
                class="mr-1.5 rounded border border-cyan-400/30 bg-cyan-400/10 px-1.5 py-px
                    font-mono text-[10px] font-semibold text-cyan-400"
            >
                {{ resolutionLabel }}
            </span>
            {{ data.total.toLocaleString() }} scenes at {{ minHeight }}p or more
        </HomepageWidgetsScenesToolbar>
        <HomepageWidgetsScenesEmpty
            v-if="data.scenes.length === 0"
            icon="heroicons:tv"
            :message="`No scene at ${minHeight}p or more yet.`"
        />
        <HomepageSectionGrid
            v-else
            :scenes="data.scenes"
            :watch-progress="data.watch_progress"
            :ratings="data.ratings"
        />
    </div>
</template>
