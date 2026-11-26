<script setup lang="ts">
import type { SceneListItem, SceneListResponse } from '~/types/scene';
import type { WidgetMarker } from '~/types/homepageMarkerWidgets';
import type { ProfileEdit, ShowcaseItem } from '~/types/profile';

// Editor of a showcase (favorite videos or favorite markers): the picked items in order and a
// search to add more. Each change is saved at once.
const props = defineProps<{ state: ProfileEdit; kind: 'scenes' | 'markers' }>();
const emit = defineEmits<{ saved: [] }>();

const api = useApiProfiles();
const scenesApi = useApiScenes();
const scenes = ref<SceneListItem[]>([]);
const markers = ref<WidgetMarker[]>([]);
const query = ref('');
const results = ref<ShowcaseItem[]>([]);
const searching = ref(false);
const saving = ref(false);
const error = ref('');

const max = computed(() => props.state.limits.max_showcase_items);

function sync() {
    scenes.value = props.state.favorite_scenes;
    markers.value = props.state.favorite_markers;
}
watch(() => props.state, sync);
onMounted(sync);

const sceneItem = (s: SceneListItem): ShowcaseItem => ({
    id: s.id,
    title: s.title || `Scene ${s.id}`,
    thumb: s.thumbnail_path ? `/thumbnails/${s.id}?size=sm` : null,
});
const markerItem = (m: WidgetMarker): ShowcaseItem => ({
    id: m.id,
    title: `${m.label || 'No label'} - ${m.scene_title}`,
    thumb: m.has_thumbnail ? markerThumbnailUrl(m.id, m.updated_at) : null,
});

const items = computed(() =>
    props.kind === 'scenes' ? scenes.value.map(sceneItem) : markers.value.map(markerItem),
);
const picked = computed(() => new Set(items.value.map((i) => i.id)));

async function save(ids: number[]) {
    saving.value = true;
    error.value = '';
    try {
        if (props.kind === 'scenes') scenes.value = (await api.setFavoriteScenes(ids)).scenes;
        else markers.value = (await api.setFavoriteMarkers(ids)).markers;
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the showcase';
    } finally {
        saving.value = false;
    }
}

function add(id: number) {
    if (picked.value.has(id) || items.value.length >= max.value) return;
    save([...items.value.map((i) => i.id), id]);
}

let timer: ReturnType<typeof setTimeout> | undefined;
let searchSeq = 0;
async function search() {
    const seq = ++searchSeq;
    searching.value = true;
    try {
        let found: ShowcaseItem[];
        if (props.kind === 'scenes') {
            const res = (await scenesApi.searchScenes({
                q: query.value,
                limit: 12,
            })) as SceneListResponse;
            found = res.data.map(sceneItem);
        } else {
            found = (await api.listMarkerCandidates(query.value, 1, 12)).data.map(markerItem);
        }
        if (seq === searchSeq) results.value = found;
    } catch (e: unknown) {
        if (seq === searchSeq) error.value = e instanceof Error ? e.message : 'Search failed';
    } finally {
        if (seq === searchSeq) searching.value = false;
    }
}
watch(query, () => {
    clearTimeout(timer);
    timer = setTimeout(search, 300);
});
onMounted(search);
onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
    <div class="space-y-3">
        <ProfileEditShowcaseList :items="items" :max="max" :disabled="saving" @change="save" />
        <input
            v-model="query"
            type="search"
            :placeholder="
                kind === 'scenes' ? 'Search videos to add' : 'Search markers by label or video'
            "
            class="border-border focus:border-lava/50 w-full rounded-md border bg-white/[0.03] px-3
                py-1.5 text-xs text-white outline-none"
        />
        <div class="grid grid-cols-3 gap-2 sm:grid-cols-6" :class="searching ? 'opacity-60' : ''">
            <button
                v-for="r in results"
                :key="r.id"
                class="border-border hover:border-lava/50 overflow-hidden rounded-md border
                    bg-black/40 text-left transition-colors disabled:opacity-40"
                :disabled="picked.has(r.id) || items.length >= max || saving"
                :title="r.title"
                @click="add(r.id)"
            >
                <div class="aspect-video w-full">
                    <img
                        v-if="r.thumb"
                        :src="r.thumb"
                        alt=""
                        loading="lazy"
                        class="h-full w-full object-cover"
                    />
                </div>
                <div class="truncate px-1.5 py-1 text-[10px] text-white">{{ r.title }}</div>
            </button>
        </div>
        <p v-if="!searching && results.length === 0" class="text-dim text-[11px]">No match.</p>
        <p v-if="error" class="text-lava text-[11px]">{{ error }}</p>
    </div>
</template>
