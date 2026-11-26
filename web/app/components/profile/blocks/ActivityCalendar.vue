<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import type { ProfileBlock } from '~/types/profile';

// activity_calendar: the heatmap of the owner's views and/or O (profile block). The server
// reuses the activity_calendar homepage widget, so its component renders it.
const props = defineProps<{ block: ProfileBlock; owner: boolean }>();

const section = computed<HomepageSection>(() => ({
    id: 'profile-activity-calendar',
    type: 'activity_calendar',
    title: props.block.label,
    enabled: true,
    limit: 0,
    order: 0,
    sort: '',
    config: { ...props.block.config },
}));

// The payload is the widget's own (CalendarPayload, typed by its component)
const data = computed<HomepageSectionData<never>>(() => ({
    section: section.value,
    scenes: [],
    total: 0,
    widget: props.block.widget as never,
}));
</script>

<template>
    <HomepageWidgetsActivityCalendar :section="section" :data="data" />
</template>
