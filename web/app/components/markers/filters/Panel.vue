<script setup lang="ts">
import type { MarkerFilterState } from '~/composables/useMarkerFilters';
import type { SceneFilterOptions } from '~/types/scene';

// Advanced filter panel of the marker pages (shown when "Advanced Marker Filters" is on).
// The filter state object is owned by useMarkerFilters() and edited in place.
const props = defineProps<{
    state: MarkerFilterState;
    options: SceneFilterOptions | null;
    activeCount: number;
    withLabels?: boolean;
}>();

const emit = defineEmits<{
    reset: [];
}>();

const filters = toRef(props, 'state');

const labelItems = computed(() =>
    (props.options?.marker_labels ?? []).map((l) => ({
        value: l.label,
        label: l.label,
        count: l.count,
    })),
);
// Marker tags share the tag list; counts are scene counts, so they are not shown
const markerTagItems = computed(() =>
    (props.options?.tags ?? []).map((t) => ({ value: t.name, label: t.name, color: t.color })),
);
const tagItems = computed(() =>
    (props.options?.tags ?? []).map((t) => ({
        value: t.name,
        label: t.name,
        count: t.scene_count,
        color: t.color,
    })),
);
const actorItems = computed(() =>
    (props.options?.actors ?? []).map((a) => ({ value: a, label: a })),
);
const studioItems = computed(() =>
    (props.options?.studios ?? []).map((s) => ({ value: s, label: s })),
);

const rangesOnly = computed({
    get: () => filters.value.rangesOnly,
    set: (v: boolean) => (filters.value.rangesOnly = v),
});
const markerTags = computed({
    get: () => filters.value.markerTags,
    set: (v: string[]) => (filters.value.markerTags = v),
});
const labels = computed({
    get: () => filters.value.labels,
    set: (v: string[]) => (filters.value.labels = v),
});
const tags = computed({
    get: () => filters.value.tags,
    set: (v: string[]) => (filters.value.tags = v),
});
const actors = computed({
    get: () => filters.value.actors,
    set: (v: string[]) => (filters.value.actors = v),
});
const studio = computed({
    get: () => (filters.value.studio ? [filters.value.studio] : []),
    set: (v: string[]) => (filters.value.studio = v[0] ?? ''),
});
</script>

<template>
    <div class="border-border bg-panel/60 rounded-lg border p-3 backdrop-blur-sm">
        <div class="grid grid-cols-1 items-start gap-2 sm:grid-cols-2 lg:grid-cols-6">
            <!-- Range markers only -->
            <div class="border-border flex items-center justify-between rounded-md border px-2.5">
                <div class="flex items-center gap-1.5 py-2">
                    <Icon name="heroicons:arrows-right-left" size="12" class="text-dim" />
                    <span class="text-dim text-[11px] font-medium uppercase">With end only</span>
                </div>
                <UiToggle v-model="rangesOnly" />
            </div>

            <MarkersFiltersPick
                v-if="withLabels"
                v-model="labels"
                title="Labels"
                icon="heroicons:bookmark"
                :items="labelItems"
            />
            <MarkersFiltersPick
                v-model="markerTags"
                title="Marker Tags"
                icon="heroicons:bookmark-square"
                :items="markerTagItems"
            />
            <MarkersFiltersPick
                v-model="tags"
                title="Scene Tags"
                icon="heroicons:tag"
                :items="tagItems"
            />
            <MarkersFiltersPick
                v-model="actors"
                title="Actors"
                icon="heroicons:user"
                :items="actorItems"
            />
            <MarkersFiltersPick
                v-model="studio"
                title="Studio"
                icon="heroicons:building-office"
                :items="studioItems"
                single
            />
        </div>
        <div v-if="activeCount > 0" class="mt-2 flex justify-end">
            <button
                class="text-lava hover:text-lava/80 text-[11px] font-medium transition-colors"
                @click="emit('reset')"
            >
                Reset filters
            </button>
        </div>
    </div>
</template>
