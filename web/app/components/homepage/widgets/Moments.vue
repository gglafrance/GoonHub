<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import type { MarkerWidgetPayload } from '~/types/homepageMarkerWidgets';

// moments widget: horizontal carousel of marker previews.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const payload = computed(() => props.data.widget as MarkerWidgetPayload | undefined);
const items = computed(() => payload.value?.items ?? []);

const track = ref<HTMLElement | null>(null);

const scrollBy = (direction: number) => {
    const el = track.value;
    if (!el) return;
    el.scrollBy({ left: direction * el.clientWidth * 0.8, behavior: 'smooth' });
};
</script>

<template>
    <div>
        <div
            v-if="items.length === 0"
            class="border-border text-dim flex h-32 items-center justify-center rounded-lg border
                border-dashed text-xs"
        >
            No markers to show
        </div>
        <div v-else class="relative">
            <div
                ref="track"
                class="flex snap-x gap-3 overflow-x-auto scroll-smooth pb-2"
                style="scrollbar-width: thin"
            >
                <HomepageWidgetsMarkersMarkerCard
                    v-for="marker in items"
                    :key="marker.id"
                    :marker="marker"
                    class="w-56 shrink-0 snap-start sm:w-64"
                />
            </div>
            <div class="mt-1 flex items-center justify-end gap-1">
                <button
                    v-if="payload?.order === 'random'"
                    class="border-border text-dim hover:border-lava/30 hover:text-lava mr-auto flex
                        items-center gap-1 rounded-md border px-2 py-1 text-[11px] transition-all"
                    title="Show other moments"
                    @click="emit('refresh')"
                >
                    <Icon name="heroicons:arrow-path" size="12" />
                    Shuffle
                </button>
                <button
                    class="border-border text-dim flex h-7 w-7 items-center justify-center
                        rounded-md border transition-colors hover:text-white"
                    aria-label="Previous"
                    @click="scrollBy(-1)"
                >
                    <Icon name="heroicons:chevron-left" size="14" />
                </button>
                <button
                    class="border-border text-dim flex h-7 w-7 items-center justify-center
                        rounded-md border transition-colors hover:text-white"
                    aria-label="Next"
                    @click="scrollBy(1)"
                >
                    <Icon name="heroicons:chevron-right" size="14" />
                </button>
            </div>
        </div>
    </div>
</template>
