<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import type { SceneListItem } from '~/types/scene';

// on_this_day widget: scenes watched or added on this date in past years, grouped by year.
interface OnThisDayGroup {
    year: number;
    watched: number[];
    added: number[];
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ date: string; groups: OnThisDayGroup[] }>;
}>();

const scenesById = computed(() => new Map(props.data.scenes.map((s) => [s.id, s])));
const currentYear = new Date().getFullYear();

const groups = computed(() =>
    (props.data.widget?.groups ?? []).map((g) => {
        const ids = [...new Set([...g.watched, ...g.added])];
        return {
            year: g.year,
            ago: currentYear - g.year,
            watchedCount: g.watched.length,
            addedCount: g.added.length,
            scenes: ids
                .map((id) => scenesById.value.get(id))
                .filter((s): s is SceneListItem => !!s),
        };
    }),
);

const dateLabel = computed(() => {
    const [month, day] = (props.data.widget?.date ?? '').split('-').map(Number);
    if (!month || !day) return 'this day';
    return new Date(2000, month - 1, day).toLocaleDateString(undefined, {
        month: 'long',
        day: 'numeric',
    });
});
</script>

<template>
    <div>
        <HomepageWidgetsScenesEmpty
            v-if="groups.length === 0"
            icon="heroicons:calendar-days"
            :message="`Nothing watched or added on ${dateLabel} in previous years.`"
        />
        <template v-else>
            <div v-for="group in groups" :key="group.year" class="mb-4 last:mb-0">
                <HomepageWidgetsScenesToolbar>
                    <span class="font-semibold text-white">{{ group.year }}</span>
                    · {{ group.ago }} year{{ group.ago > 1 ? 's' : '' }} ago
                    <template v-if="group.watchedCount">
                        · {{ group.watchedCount }} watched</template
                    >
                    <template v-if="group.addedCount"> · {{ group.addedCount }} added</template>
                </HomepageWidgetsScenesToolbar>
                <HomepageSectionGrid :scenes="group.scenes" :ratings="data.ratings" />
            </div>
        </template>
    </div>
</template>
