<script setup lang="ts">
import type { Tag } from '~/types/tag';
import type { ActorListItem } from '~/types/actor';
import type { StudioListItem } from '~/types/studio';
import type { TriageSceneDetails } from '~/composables/useTriageSceneDetails';
import type { TriageOption } from '~/types/triage';

// Tags, actors and studio of the triaged scene, each with a keyboard-first input.
const props = defineProps<{
    details: TriageSceneDetails;
    actions: ReturnType<typeof useTriageActions>;
}>();

const { fetchTags } = useApiTags();
const { fetchActors } = useApiActors();
const { fetchStudios } = useApiStudios();

const tagField = ref<{ focus: () => void } | null>(null);
const actorField = ref<{ focus: () => void } | null>(null);
const studioField = ref<{ focus: () => void } | null>(null);

const SUGGESTIONS = 8;
const number = new Intl.NumberFormat();
const countHint = (n?: number) => (n ? number.format(n) : undefined);

// All tags are loaded once (shared between scenes, refreshed after a while) and filtered
// locally
const TAGS_TTL_MS = 5 * 60 * 1000;
const allTags = useState<Tag[]>('triage-all-tags', () => []);
const tagsLoadedAt = useState<number>('triage-all-tags-at', () => 0);
let tagsLoading: Promise<void> | null = null;
const ensureTags = () => {
    if (tagsLoadedAt.value && Date.now() - tagsLoadedAt.value < TAGS_TTL_MS) {
        return Promise.resolve();
    }
    if (!tagsLoading) {
        tagsLoading = fetchTags()
            .then((res) => {
                allTags.value = res.data || [];
                tagsLoadedAt.value = Date.now();
            })
            .catch(() => {})
            .finally(() => {
                tagsLoading = null;
            });
    }
    return tagsLoading;
};

const searchTags = async (query: string): Promise<TriageOption[]> => {
    await ensureTags();
    const q = query.toLowerCase();
    return allTags.value
        .filter((t) => !q || t.name.toLowerCase().includes(q))
        .sort((a, b) => {
            if (q) {
                const pa = a.name.toLowerCase().startsWith(q) ? 0 : 1;
                const pb = b.name.toLowerCase().startsWith(q) ? 0 : 1;
                if (pa !== pb) return pa - pb;
            }
            return b.scene_count - a.scene_count || a.name.localeCompare(b.name);
        })
        .slice(0, SUGGESTIONS + props.details.tags.length)
        .map((t) => ({ id: t.id, name: t.name, hint: countHint(t.scene_count) }));
};

const searchActors = async (query: string): Promise<TriageOption[]> => {
    const res = await fetchActors(1, SUGGESTIONS + props.details.actors.length, query || undefined);
    return ((res.data || []) as ActorListItem[]).map((a) => ({
        id: a.id,
        name: a.name,
        hint: countHint(a.scene_count),
    }));
};

const searchStudios = async (query: string): Promise<TriageOption[]> => {
    const res = await fetchStudios(1, SUGGESTIONS + 1, query || undefined);
    return ((res.data || []) as StudioListItem[]).map((s) => ({
        id: s.id,
        name: s.name,
        hint: countHint(s.scene_count),
    }));
};

const onAddTag = (option: TriageOption) => {
    const tag = allTags.value.find((t) => t.id === option.id);
    if (tag) void props.actions.addTag(tag);
};

const onCreateTag = async (name: string) => {
    const tag = await props.actions.createTag(name);
    if (tag) allTags.value = [...allTags.value, { ...tag, scene_count: 1 }];
};

const studioItems = computed(() =>
    props.details.studio ? [{ id: props.details.studio.id, name: props.details.studio.name }] : [],
);

const tagLink = (item: { name: string }) => `/search?tags=${encodeURIComponent(item.name)}`;
const actorLink = (item: { id: number }) => {
    const actor = props.details.actors.find((a) => a.id === item.id);
    return actor?.uuid ? `/actors/${actor.uuid}` : undefined;
};
const studioLink = () =>
    props.details.studio?.uuid ? `/studios/${props.details.studio.uuid}` : undefined;

defineExpose({
    focusTags: () => tagField.value?.focus(),
    focusActors: () => actorField.value?.focus(),
    focusStudio: () => studioField.value?.focus(),
});
</script>

<template>
    <div class="border-border bg-surface/60 space-y-4 rounded-xl border p-4 backdrop-blur-sm">
        <TriageEntityField
            ref="tagField"
            label="Tags"
            icon="heroicons:tag"
            shortcut="T"
            :items="details.tags"
            :can-edit="actions.canEdit.value"
            :allow-create="actions.canEdit.value"
            :search="searchTags"
            :link-for="tagLink"
            placeholder="Add a tag..."
            @add="onAddTag"
            @create="onCreateTag"
            @remove="actions.removeTag"
        />

        <TriageEntityField
            ref="actorField"
            label="Actors"
            icon="heroicons:user"
            shortcut="A"
            :items="details.actors"
            :can-edit="actions.canEdit.value"
            :allow-create="actions.canCreateEntities.value"
            :search="searchActors"
            :link-for="actorLink"
            placeholder="Add an actor..."
            @add="actions.addActor"
            @create="actions.createActor"
            @remove="actions.removeActor"
        />

        <TriageEntityField
            ref="studioField"
            label="Studio"
            icon="heroicons:building-office-2"
            shortcut="S"
            :items="studioItems"
            :can-edit="actions.canEdit.value"
            :allow-create="actions.canCreateEntities.value"
            :search="searchStudios"
            :link-for="studioLink"
            placeholder="Set the studio..."
            single
            @add="actions.setStudio"
            @create="actions.createStudio"
            @remove="actions.clearStudio"
        />

        <p v-if="!actions.canEdit.value" class="text-dim text-[11px]">
            You can rate and like scenes, but editing tags, actors and studios needs the
            "scenes:upload" permission.
        </p>

        <button
            v-if="actions.canTrash.value"
            class="border-border text-dim hover:border-lava/40 hover:bg-lava/10 hover:text-lava flex
                w-full items-center justify-center gap-1.5 rounded-md border py-1.5 text-[11px]
                font-medium transition-all"
            title="Move to trash (Delete)"
            tabindex="-1"
            @click="actions.trash"
        >
            <Icon name="heroicons:trash" size="13" />
            Move to trash
        </button>
    </div>
</template>
