<script setup lang="ts">
import type { ActorMergePreview } from '~/types/actor';
import type { StudioMergePreview } from '~/types/studio';

// "Merge into...": pick the actor/studio to keep, review what moves, confirm.
// The source (the entity of the current page) is merged into the picked target and deleted.
interface Candidate {
    id: number;
    uuid: string;
    name: string;
    scene_count: number;
    image: string;
    matched_alias?: string;
}

const props = defineProps<{
    visible: boolean;
    kind: 'actor' | 'studio';
    source: { id: number; uuid: string; name: string };
}>();

const emit = defineEmits<{
    close: [];
    merged: [targetUuid: string];
}>();

const { fetchActors, fetchActorMergePreview, mergeActor } = useApiActors();
const { fetchStudios, fetchStudioMergePreview, mergeStudio } = useApiStudios();

const step = ref<'pick' | 'confirm'>('pick');
const query = ref('');
const candidates = ref<Candidate[]>([]);
const loadingCandidates = ref(false);
const target = ref<Candidate | null>(null);
const preview = ref<ActorMergePreview | StudioMergePreview | null>(null);
const loadingPreview = ref(false);
const merging = ref(false);
const error = ref('');
const searchInputRef = ref<HTMLInputElement | null>(null);
let searchTimeout: ReturnType<typeof setTimeout> | null = null;

const noun = computed(() => props.kind);
const actorPreview = computed(() =>
    props.kind === 'actor' ? (preview.value as ActorMergePreview | null) : null,
);
const studioPreview = computed(() =>
    props.kind === 'studio' ? (preview.value as StudioMergePreview | null) : null,
);
const keepsOwnImage = computed(() =>
    actorPreview.value
        ? !actorPreview.value.image_from_source
        : !studioPreview.value?.logo_from_source,
);

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`;

const loadCandidates = async () => {
    loadingCandidates.value = true;
    error.value = '';
    try {
        const q = query.value.trim() || undefined;
        if (props.kind === 'actor') {
            const res = await fetchActors(1, 30, q, 'name_asc');
            candidates.value = (res.data || []).map(
                (a: {
                    id: number;
                    uuid: string;
                    name: string;
                    scene_count: number;
                    image_url: string;
                    matched_alias?: string;
                }) => ({
                    id: a.id,
                    uuid: a.uuid,
                    name: a.name,
                    scene_count: a.scene_count,
                    image: a.image_url,
                    matched_alias: a.matched_alias,
                }),
            );
        } else {
            const res = await fetchStudios(1, 30, q, 'name_asc');
            candidates.value = (res.data || []).map(
                (s: {
                    id: number;
                    uuid: string;
                    name: string;
                    scene_count: number;
                    logo: string;
                    matched_alias?: string;
                }) => ({
                    id: s.id,
                    uuid: s.uuid,
                    name: s.name,
                    scene_count: s.scene_count,
                    image: s.logo,
                    matched_alias: s.matched_alias,
                }),
            );
        }
        candidates.value = candidates.value.filter((c) => c.id !== props.source.id);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : `Failed to load ${noun.value}s`;
    } finally {
        loadingCandidates.value = false;
    }
};

watch(query, () => {
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(loadCandidates, 300);
});

watch(
    () => props.visible,
    (open) => {
        if (open) {
            step.value = 'pick';
            query.value = props.source.name;
            target.value = null;
            preview.value = null;
            error.value = '';
            loadCandidates();
            nextTick(() => searchInputRef.value?.select());
        }
    },
    { immediate: true },
);

const pickTarget = async (candidate: Candidate) => {
    target.value = candidate;
    step.value = 'confirm';
    preview.value = null;
    loadingPreview.value = true;
    error.value = '';
    try {
        preview.value =
            props.kind === 'actor'
                ? await fetchActorMergePreview(candidate.uuid, props.source.uuid)
                : await fetchStudioMergePreview(candidate.uuid, props.source.uuid);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to preview the merge';
    } finally {
        loadingPreview.value = false;
    }
};

const backToPick = () => {
    step.value = 'pick';
    target.value = null;
    preview.value = null;
    error.value = '';
};

const confirmMerge = async () => {
    if (!target.value) return;
    merging.value = true;
    error.value = '';
    try {
        if (props.kind === 'actor') {
            await mergeActor(target.value.uuid, props.source.uuid);
        } else {
            await mergeStudio(target.value.uuid, props.source.uuid);
        }
        emit('merged', target.value.uuid);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Merge failed';
    } finally {
        merging.value = false;
    }
};

const onKeydown = (e: KeyboardEvent) => {
    if (e.key === 'Escape' && !merging.value) emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4
                backdrop-blur-sm"
            @click.self="!merging && emit('close')"
            @keydown="onKeydown"
        >
            <div
                class="glass-panel border-border flex max-h-[80dvh] w-full max-w-md flex-col border"
            >
                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 py-3">
                    <div class="flex min-w-0 items-center gap-2">
                        <Icon
                            name="heroicons:arrows-pointing-in"
                            size="16"
                            class="text-lava shrink-0"
                        />
                        <h3 class="truncate text-sm font-semibold text-white">
                            Merge "{{ source.name }}" into...
                        </h3>
                    </div>
                    <button
                        type="button"
                        class="text-dim transition-colors hover:text-white"
                        :disabled="merging"
                        @click="emit('close')"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Step 1: pick the target -->
                <template v-if="step === 'pick'">
                    <div class="px-4 pt-3">
                        <p class="text-dim mb-2 text-xs">
                            Pick the {{ noun }} to keep. "{{ source.name }}" will be merged into it,
                            then deleted.
                        </p>
                        <div class="relative">
                            <Icon
                                name="heroicons:magnifying-glass"
                                size="14"
                                class="text-dim absolute top-1/2 left-3 -translate-y-1/2"
                            />
                            <input
                                ref="searchInputRef"
                                v-model="query"
                                type="text"
                                :placeholder="`Search ${noun}s by name or alias...`"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 w-full rounded-lg border py-2 pr-3 pl-8
                                    text-sm text-white focus:outline-none"
                            />
                        </div>
                    </div>
                    <div class="min-h-40 flex-1 overflow-y-auto px-2 py-2">
                        <div v-if="loadingCandidates" class="flex h-32 items-center justify-center">
                            <LoadingSpinner />
                        </div>
                        <p
                            v-else-if="candidates.length === 0"
                            class="text-dim py-8 text-center text-xs"
                        >
                            No other {{ noun }} matches
                        </p>
                        <button
                            v-for="c in candidates"
                            v-else
                            :key="c.id"
                            type="button"
                            class="group flex w-full items-center gap-3 rounded-lg px-2 py-1.5
                                text-left transition-colors hover:bg-white/5"
                            @click="pickTarget(c)"
                        >
                            <div
                                class="bg-surface border-border flex h-9 w-9 shrink-0 items-center
                                    justify-center overflow-hidden rounded-md border"
                            >
                                <img
                                    v-if="c.image"
                                    :src="c.image"
                                    :alt="c.name"
                                    class="h-full w-full"
                                    :class="
                                        kind === 'actor' ? 'object-cover' : 'object-contain p-1'
                                    "
                                />
                                <Icon
                                    v-else
                                    :name="
                                        kind === 'actor'
                                            ? 'heroicons:user'
                                            : 'heroicons:building-office-2'
                                    "
                                    size="16"
                                    class="text-dim"
                                />
                            </div>
                            <div class="min-w-0 flex-1">
                                <p
                                    class="truncate text-xs font-medium text-white/80
                                        group-hover:text-white"
                                >
                                    {{ c.name }}
                                </p>
                                <p class="text-dim truncate text-[10px]">
                                    {{ plural(c.scene_count || 0, 'scene') }}
                                    <span v-if="c.matched_alias" class="text-lava/80">
                                        · aka {{ c.matched_alias }}
                                    </span>
                                </p>
                            </div>
                            <Icon
                                name="heroicons:chevron-right"
                                size="14"
                                class="text-dim group-hover:text-lava shrink-0"
                            />
                        </button>
                    </div>
                </template>

                <!-- Step 2: what moves, confirm -->
                <template v-else>
                    <div class="flex-1 overflow-y-auto px-4 py-3">
                        <div v-if="loadingPreview" class="flex h-32 items-center justify-center">
                            <LoadingSpinner label="Checking what moves..." />
                        </div>
                        <div v-else-if="preview && target" class="space-y-3 text-xs">
                            <p class="text-white/80">
                                <span class="font-semibold text-white">{{ source.name }}</span>
                                will be merged into
                                <span class="text-lava font-semibold">{{ target.name }}</span
                                >:
                            </p>
                            <ul
                                class="border-border bg-surface/60 space-y-1.5 rounded-lg border
                                    p-3"
                            >
                                <li class="flex items-start gap-2">
                                    <Icon name="heroicons:film" size="14" class="text-dim mt-px" />
                                    <span>
                                        {{ plural(preview.scenes, 'scene') }} move to
                                        {{ target.name }}
                                        <span
                                            v-if="actorPreview && actorPreview.shared_scenes > 0"
                                            class="text-dim"
                                        >
                                            ({{ actorPreview.shared_scenes }} already credit them)
                                        </span>
                                    </span>
                                </li>
                                <li
                                    v-if="studioPreview && studioPreview.child_studios > 0"
                                    class="flex items-start gap-2"
                                >
                                    <Icon
                                        name="heroicons:building-office-2"
                                        size="14"
                                        class="text-dim mt-px"
                                    />
                                    <span>
                                        {{ plural(studioPreview.child_studios, 'child studio') }}
                                        move under {{ target.name }}
                                    </span>
                                </li>
                                <li class="flex items-start gap-2">
                                    <Icon name="heroicons:star" size="14" class="text-dim mt-px" />
                                    <span>
                                        {{ plural(preview.ratings, 'rating') }} and
                                        {{ plural(preview.likes, 'like') }} move
                                        <span class="text-dim">
                                            (a user's own rating of {{ target.name }} is kept)
                                        </span>
                                    </span>
                                </li>
                                <li class="flex items-start gap-2">
                                    <Icon name="heroicons:photo" size="14" class="text-dim mt-px" />
                                    <span v-if="keepsOwnImage">
                                        {{ target.name }} keeps its
                                        {{ kind === 'actor' ? 'photo' : 'logo' }}
                                    </span>
                                    <span v-else>
                                        {{ target.name }} takes the
                                        {{ kind === 'actor' ? 'photo' : 'logo' }} of
                                        {{ source.name }}
                                    </span>
                                </li>
                                <li class="flex items-start gap-2">
                                    <Icon name="heroicons:tag" size="14" class="text-dim mt-px" />
                                    <div class="min-w-0">
                                        <span v-if="preview.aliases_added.length === 0">
                                            No new alias
                                        </span>
                                        <template v-else>
                                            <span>New aliases of {{ target.name }}:</span>
                                            <div class="mt-1 flex flex-wrap gap-1">
                                                <span
                                                    v-for="alias in preview.aliases_added"
                                                    :key="alias"
                                                    class="border-border bg-panel rounded-full
                                                        border px-2 py-0.5 text-[11px] text-white"
                                                >
                                                    {{ alias }}
                                                </span>
                                            </div>
                                        </template>
                                    </div>
                                </li>
                            </ul>
                            <p class="flex items-start gap-1.5 text-red-300">
                                <Icon
                                    name="heroicons:exclamation-triangle"
                                    size="14"
                                    class="mt-px shrink-0"
                                />
                                "{{ source.name }}" is then deleted. This cannot be undone.
                            </p>
                        </div>
                    </div>
                    <div class="border-border flex justify-end gap-2 border-t px-4 py-3">
                        <button
                            type="button"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            :disabled="merging"
                            @click="backToPick"
                        >
                            Back
                        </button>
                        <button
                            type="button"
                            :disabled="merging || loadingPreview || !preview"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                            @click="confirmMerge"
                        >
                            {{ merging ? 'Merging...' : `Merge into ${target?.name ?? ''}` }}
                        </button>
                    </div>
                </template>

                <p v-if="error" class="text-lava border-border border-t px-4 py-2 text-xs">
                    {{ error }}
                </p>
            </div>
        </div>
    </Teleport>
</template>
