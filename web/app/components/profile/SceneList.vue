<script setup lang="ts">
import type {
    ProfileListKind,
    ProfileListResponse,
    ProfileScene,
    ProfileSceneFigureKind,
} from '~/types/profile';
import type { LoadedPages, PageLoadPlan } from '~/utils/infinitePages';

// A full list of a profile (liked, rated or jizz videos, docs/PROFILES.md "Lists"): the user's
// card template and grid density, paginated or infinite (user setting), page and sort in the
// URL. Under each card: the owner's figure; the card badges are the viewer's own interactions.
const props = defineProps<{
    username: string;
    /** A list of the profile, or a custom list (`lists/<id>`) */
    kind: ProfileListKind | `lists/${number}`;
    sortOptions: { value: string; label: string }[];
    figure: ProfileSceneFigureKind;
    atLabel?: string;
    /** Ratings list: one bar of the chart (null: every rating) */
    rating?: number | null;
    emptyText: string;
}>();

const emit = defineEmits<{ total: [total: number]; loaded: [response: ProfileListResponse] }>();

const api = useApiProfiles();
const settingsStore = useSettingsStore();
const { sceneGridClass, sceneGridStyle } = useGridDensity();

const page = useUrlPagination();
const sort = useUrlSort(props.sortOptions[0]?.value ?? '');
const limit = computed(() => settingsStore.videosPerPage);

const items = ref<ProfileScene[]>([]);
const total = ref(0);
const sidecars = ref(emptyCardSidecars());
const loading = ref(true);
const loadingMore = ref(false);
const error = ref('');
const loadedPages = ref<LoadedPages>({ first: 0, last: 0 });
let request = 0;

const context = () =>
    JSON.stringify([props.username, props.kind, sort.value, props.rating ?? null, limit.value]);

async function run(plan: PageLoadPlan) {
    const current = ++request;
    const partial = plan.mode !== 'replace';
    if (partial) loadingMore.value = true;
    else loading.value = true;
    error.value = '';
    try {
        const responses = await Promise.all(
            plan.pages.map((p) =>
                api.listProfileScenes(props.username, props.kind, {
                    page: p,
                    limit: limit.value,
                    sort: sort.value || undefined,
                    rating: props.rating ?? null,
                    cardFields: settingsStore.listFieldsParam,
                }),
            ),
        );
        if (current !== request) return;
        // mergePageItems works on ids: key the items by their scene
        const keyed = (list: ProfileScene[]) => list.map((i) => ({ ...i, id: i.scene.id }));
        items.value = mergePageItems(
            keyed(items.value),
            keyed(responses.flatMap((r) => r.data)),
            plan.mode,
        );
        total.value = responses[responses.length - 1]?.pagination.total_items ?? 0;
        sidecars.value = mergeCardSidecars(sidecars.value, responses, partial);
        loadedPages.value = commitPageLoad(loadedPages.value, plan);
        emit('total', total.value);
        const last = responses[responses.length - 1];
        if (last) emit('loaded', last);
    } catch (e: unknown) {
        if (current !== request) return;
        error.value = e instanceof Error ? e.message : 'Failed to load the videos';
        if (!partial) items.value = [];
    } finally {
        if (current === request) {
            loading.value = false;
            loadingMore.value = false;
        }
    }
}

const load = (p: number) =>
    run(planPageLoad(loadedPages.value, p, settingsStore.infiniteScroll, context()));

const { keepPositionWhile } = useInfiniteScrollPosition();
const hasMore = computed(() => loadedPages.value.last * limit.value < total.value);
const loadMore = () => {
    if (loading.value || loadingMore.value) return;
    page.value = loadedPages.value.last + 1;
};
const loadPrevious = () =>
    keepPositionWhile(async () => {
        const plan = planPreviousPage(loadedPages.value);
        if (plan) await run(plan);
    });

// A new filter or sort starts again from the first page
const restart = () => {
    if (page.value === 1) load(1);
    else page.value = 1;
};
watch(page, (p) => load(p));
watch(sort, restart);
watch(() => [props.username, props.rating], restart);
watch(
    () => settingsStore.infiniteScroll,
    () => load(page.value),
);
// Cards of a template with new fields need them
watch(
    () => settingsStore.listFieldsParam,
    () => load(page.value),
);
onMounted(() => load(page.value));
</script>

<template>
    <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
            <slot name="toolbar" />
            <span class="text-dim text-[11px]">
                <span class="font-mono text-white">{{ total }}</span>
                {{ total === 1 ? 'video' : 'videos' }}
            </span>
            <UiSortSelect
                v-if="props.sortOptions.length > 0"
                v-model="sort"
                :options="props.sortOptions"
                class="ml-auto"
            />
        </div>

        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>
        <div v-else-if="loading" class="flex h-32 items-center justify-center">
            <LoadingSpinner label="Loading videos..." />
        </div>
        <div
            v-else-if="items.length === 0"
            class="border-border text-dim flex h-32 flex-col items-center justify-center rounded-xl
                border border-dashed text-center text-xs"
        >
            <Icon name="heroicons:film" size="22" class="mb-2" />
            {{ props.emptyText }}
        </div>
        <div v-else>
            <Pagination
                v-if="!settingsStore.infiniteScroll"
                v-model="page"
                position="top"
                :total="total"
                :limit="limit"
            />
            <SceneListLoadPrevious
                v-else-if="loadedPages.first > 1"
                :first-page="loadedPages.first"
                :loading="loadingMore"
                @load-previous="loadPrevious"
            />
            <div
                class="scene-grid grid justify-center"
                :class="sceneGridClass"
                :style="sceneGridStyle"
            >
                <div v-for="item in items" :key="item.scene.id" class="min-w-0">
                    <SceneCard
                        :scene="item.scene"
                        :rating="sidecars.ratings[item.scene.id]"
                        :liked="sidecars.likes[item.scene.id]"
                        :jizz-count="sidecars.jizzCounts[item.scene.id]"
                        :completed="sidecars.watched[item.scene.id]"
                        fluid
                    />
                    <ProfileSceneFigure
                        :item="item"
                        :figure="props.figure"
                        :at-label="props.atLabel"
                    />
                </div>
            </div>
            <SceneListInfiniteLoader
                v-if="settingsStore.infiniteScroll"
                :has-more="hasMore"
                :loading="loadingMore"
                :total="total"
                @load-more="loadMore"
            />
            <Pagination v-else v-model="page" :total="total" :limit="limit" />
        </div>
    </div>
</template>
