<script setup lang="ts">
import type { ListFigureKind, ListFiguresSetting } from '~/types/profile';
import type { SavedSearchFilters } from '~/types/saved_search';
import type { PrivacySettings } from '~/types/social';

// The figures setting of a custom list (list editor): show the owner's figures under the cards
// or not, and which ones (auto: from the sort and the filters). Each figure is only sent to
// viewers who may see its privacy item, shown here with its current audience.
const model = defineModel<ListFiguresSetting>({ required: true });
const props = defineProps<{ filters: SavedSearchFilters; privacy: PrivacySettings }>();

const FIGURE_ITEMS: Record<ListFigureKind, string> = {
    jizz: 'jizz',
    rating: 'ratings',
    liked: 'liked',
    views: 'activity',
};

const shown = computed({
    get: () => model.value !== 'none',
    set: (on: boolean) => {
        model.value = on ? 'auto' : 'none';
    },
});
const auto = computed(() => autoListFigures(props.filters));
const options = computed(() => listFigureOptions(auto.value));
const kinds = computed(() => listFigures(model.value, props.filters));
const items = computed(() =>
    kinds.value.map((k) => {
        const item = props.privacy.items.find((i) => i.key === FIGURE_ITEMS[k]);
        const audience = props.privacy.audiences.find((a) => a.value === item?.audience);
        return { kind: k, label: item?.label ?? FIGURE_ITEMS[k], audience: audience?.label ?? '' };
    }),
);
</script>

<template>
    <div class="border-border space-y-2 rounded-lg border px-3 py-2 text-[11px]">
        <div class="flex items-center gap-2">
            <span class="text-dim flex-1">Show my figures under the cards</span>
            <UiToggle v-model="shown" title="O count, rating, like date or views" />
        </div>
        <template v-if="shown">
            <UiSelectMenu
                :model-value="model"
                :options="options"
                @update:model-value="model = String($event) as ListFiguresSetting"
            />
            <p v-if="kinds.length === 0" class="text-dim text-[10px]">
                Nothing to show for these criteria: pick a figure.
            </p>
            <p v-else class="text-dim text-[10px]">
                Each figure is only shown to who can see your
                <span v-for="(it, i) in items" :key="it.kind" class="text-white"
                    >{{ i > 0 ? ', ' : '' }}{{ it.label }}
                    <span class="text-dim">({{ it.audience }})</span></span
                >.
            </p>
        </template>
    </div>
</template>
