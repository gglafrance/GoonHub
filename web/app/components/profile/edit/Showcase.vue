<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type { WidgetMarker } from '~/types/homepageMarkerWidgets';
import type { ProfileEdit, ShowcaseItem } from '~/types/profile';
import type { PickedItem } from '~/utils/picker';

// Editor of a showcase (favorite videos or favorite markers): the picked items in order, and the
// search picker to add some (up to the free slots) or replace one. Each change is saved at once.
// The Markers tab of the picker lists the markers the user sees on scenes still in the library:
// the showcase rule of the server (GET /profiles/me/marker-candidates).
const props = defineProps<{ state: ProfileEdit; kind: 'scenes' | 'markers' }>();
const emit = defineEmits<{ saved: [] }>();

const api = useApiProfiles();
const scenes = ref<SceneListItem[]>([]);
const markers = ref<WidgetMarker[]>([]);
const saving = ref(false);
const error = ref('');
const pickerOpen = ref(false);
// Slot being replaced (single select), null when adding
const replaceIndex = ref<number | null>(null);

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
const ids = computed(() => items.value.map((i) => i.id));
const free = computed(() => Math.max(0, max.value - items.value.length));
const exclude = computed(() =>
    props.kind === 'scenes' ? { scenes: ids.value } : { markers: ids.value },
);

async function save(next: number[]) {
    saving.value = true;
    error.value = '';
    try {
        if (props.kind === 'scenes') scenes.value = (await api.setFavoriteScenes(next)).scenes;
        else markers.value = (await api.setFavoriteMarkers(next)).markers;
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the showcase';
    } finally {
        saving.value = false;
    }
}

const openPicker = (index: number | null) => {
    replaceIndex.value = index;
    pickerOpen.value = true;
};

function onPicked(picked: PickedItem[]) {
    const pickedIds =
        props.kind === 'scenes'
            ? pickedScenes(picked).map((s) => s.id)
            : pickedMarkers(picked).map((m) => m.id);
    pickerOpen.value = false;
    const first = pickedIds[0];
    if (first === undefined) return;
    const index = replaceIndex.value;
    if (index !== null) {
        const next = [...ids.value];
        next[index] = first;
        save(next);
    } else {
        save([...ids.value, ...pickedIds].slice(0, max.value));
    }
}
</script>

<template>
    <div class="space-y-3">
        <ProfileEditShowcaseList
            :items="items"
            :max="max"
            :disabled="saving"
            @change="save"
            @replace="openPicker"
        />
        <button
            class="border-lava/30 bg-lava/10 hover:bg-lava/20 flex items-center gap-1.5 rounded-lg
                border px-3 py-1.5 text-xs font-medium text-white transition-colors
                disabled:opacity-40"
            :disabled="free === 0 || saving"
            @click="openPicker(null)"
        >
            <Icon name="heroicons:plus" size="13" class="text-lava" />
            {{ kind === 'scenes' ? 'Add favorite videos' : 'Add favorite markers' }}
            <span class="text-dim font-mono text-[10px]">{{ free }} free</span>
        </button>
        <p v-if="error" class="text-lava text-[11px]">{{ error }}</p>

        <PickerModal
            :visible="pickerOpen"
            :tabs="[kind]"
            :multiple="replaceIndex === null"
            :max="free"
            :exclude="exclude"
            :title="
                replaceIndex !== null
                    ? 'Replace a favorite'
                    : kind === 'scenes'
                      ? 'Add favorite videos'
                      : 'Add favorite markers'
            "
            :confirm-label="replaceIndex !== null ? 'Replace' : 'Add'"
            @picked="onPicked"
            @close="pickerOpen = false"
        />
    </div>
</template>
