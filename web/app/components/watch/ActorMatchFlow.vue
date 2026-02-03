<script setup lang="ts">
import type { Actor } from '~/types/actor';
import type { PornDBScenePerformer, PornDBPerformerDetails } from '~/types/porndb';

const props = defineProps<{
    sceneId: number;
    performers: PornDBScenePerformer[];
}>();

const emit = defineEmits<{
    done: [];
    error: [message: string];
}>();

const api = useApi();

const performerQueue = ref<PornDBScenePerformer[]>([]);
const currentPerformerIndex = ref(0);
const actorMatchStep = ref<'search' | 'porndb' | 'create' | null>(null);
const localActorResults = ref<Actor[]>([]);
const porndbPerformerResults = ref<PornDBPerformerDetails[]>([]);
const searchingActors = ref(false);
const createdActorIds = ref<number[]>([]);

const currentPerformer = computed(() => {
    if (currentPerformerIndex.value < performerQueue.value.length) {
        return performerQueue.value[currentPerformerIndex.value];
    }
    return null;
});

async function processNextPerformer() {
    if (currentPerformerIndex.value >= performerQueue.value.length) {
        // All performers processed, link them to scene
        if (createdActorIds.value.length > 0) {
            try {
                const existing = await api.fetchSceneActors(props.sceneId);
                const existingIds = (existing.data || []).map((a: Actor) => a.id);
                const allIds = [...new Set([...existingIds, ...createdActorIds.value])];
                await api.setSceneActors(props.sceneId, allIds);
            } catch (e: unknown) {
                emit('error', e instanceof Error ? e.message : 'Failed to link actors');
                return;
            }
        }
        emit('done');
        return;
    }

    const performer = performerQueue.value[currentPerformerIndex.value];
    actorMatchStep.value = 'search';
    searchingActors.value = true;

    if (!performer || !performer.name) {
        currentPerformerIndex.value++;
        await processNextPerformer();
        return;
    }

    try {
        const result = await api.fetchActors(1, 10, performer.name);
        localActorResults.value = result.data || [];

        if (localActorResults.value.length > 0) {
            actorMatchStep.value = 'search';
        } else {
            await searchPornDBForPerformer(performer);
        }
    } catch {
        currentPerformerIndex.value++;
        await processNextPerformer();
    } finally {
        searchingActors.value = false;
    }
}

async function searchPornDBForPerformer(performer: PornDBScenePerformer) {
    searchingActors.value = true;
    try {
        const results = await api.searchPornDBPerformers(performer.name);
        porndbPerformerResults.value = results;
        actorMatchStep.value = 'porndb';
    } catch {
        actorMatchStep.value = 'create';
    } finally {
        searchingActors.value = false;
    }
}

async function selectLocalActor(actor: Actor) {
    createdActorIds.value.push(actor.id);
    currentPerformerIndex.value++;
    await processNextPerformer();
}

async function skipLocalMatch() {
    const performer = performerQueue.value[currentPerformerIndex.value];
    if (!performer) {
        currentPerformerIndex.value++;
        await processNextPerformer();
        return;
    }
    await searchPornDBForPerformer(performer);
}

async function selectPornDBPerformer(performer: PornDBPerformerDetails) {
    searchingActors.value = true;
    try {
        // Fetch full details since search results only contain basic info
        const details = await api.getPornDBPerformer(performer.id);
        const newActor = await api.createActor({
            name: details.name,
            aliases: details.aliases,
            image_url: details.image,
            gender: details.gender,
            birthday: details.birthday,
            birthplace: details.birthplace,
            ethnicity: details.ethnicity,
            nationality: details.nationality,
            height_cm: details.height,
            weight_kg: details.weight,
            measurements: details.measurements,
            cupsize: details.cupsize,
            hair_color: details.hair_colour,
            eye_color: details.eye_colour,
            tattoos: details.tattoos,
            piercings: details.piercings,
            career_start_year: details.career_start_year,
            career_end_year: details.career_end_year,
            fake_boobs: details.fake_boobs,
            same_sex_only: details.same_sex_only,
        });
        createdActorIds.value.push(newActor.id);
    } catch {
        // If fetching details fails, try basic creation
        try {
            const newActor = await api.createActor({
                name: performer.name,
                image_url: performer.image,
            });
            createdActorIds.value.push(newActor.id);
        } catch {
            // Failed to create, continue anyway
        }
    } finally {
        searchingActors.value = false;
    }
    currentPerformerIndex.value++;
    await processNextPerformer();
}

async function createActorWithMetadata() {
    const performer = performerQueue.value[currentPerformerIndex.value];
    searchingActors.value = true;
    if (!performer) {
        currentPerformerIndex.value++;
        await processNextPerformer();
        return;
    }
    try {
        // Fetch full performer details from PornDB using the scene performer's ID
        const details = await api.getPornDBPerformer(performer.id);
        const newActor = await api.createActor({
            name: details.name,
            aliases: details.aliases,
            image_url: details.image,
            gender: details.gender,
            birthday: details.birthday,
            birthplace: details.birthplace,
            ethnicity: details.ethnicity,
            nationality: details.nationality,
            height_cm: details.height,
            weight_kg: details.weight,
            measurements: details.measurements,
            cupsize: details.cupsize,
            hair_color: details.hair_colour,
            eye_color: details.eye_colour,
            tattoos: details.tattoos,
            piercings: details.piercings,
            career_start_year: details.career_start_year,
            career_end_year: details.career_end_year,
            fake_boobs: details.fake_boobs,
            same_sex_only: details.same_sex_only,
        });
        createdActorIds.value.push(newActor.id);
    } catch {
        // If fetching details fails, fall back to basic name + image
        try {
            const newActor = await api.createActor({
                name: performer.name,
                image_url: performer.image,
            });
            createdActorIds.value.push(newActor.id);
        } catch {
            // Failed to create, continue anyway
        }
    } finally {
        searchingActors.value = false;
    }
    currentPerformerIndex.value++;
    await processNextPerformer();
}

async function skipPerformer() {
    currentPerformerIndex.value++;
    await processNextPerformer();
}

onMounted(() => {
    performerQueue.value = [...props.performers];
    currentPerformerIndex.value = 0;
    processNextPerformer();
});
</script>

<template>
    <div class="space-y-4">
        <!-- Progress bar -->
        <div class="space-y-2">
            <div class="flex items-center justify-between">
                <p class="text-dim text-xs">
                    Matching performer {{ currentPerformerIndex + 1 }} of
                    {{ performerQueue.length }}
                </p>
                <div class="bg-void h-1.5 w-32 overflow-hidden rounded-full">
                    <div
                        class="bg-lava h-full rounded-full transition-all duration-300"
                        :style="{
                            width: `${(currentPerformerIndex / performerQueue.length) * 100}%`,
                        }"
                    />
                </div>
            </div>
        </div>

        <!-- Current performer header -->
        <div
            v-if="currentPerformer"
            class="border-border bg-surface flex items-center gap-4 rounded-lg border p-4"
        >
            <div class="bg-void h-20 w-20 shrink-0 overflow-hidden rounded-lg">
                <img
                    v-if="currentPerformer.image"
                    :src="currentPerformer.image"
                    :alt="currentPerformer.name"
                    class="h-full w-full object-cover"
                />
                <div v-else class="text-dim flex h-full w-full items-center justify-center">
                    <Icon name="heroicons:user" size="28" />
                </div>
            </div>
            <div>
                <p class="text-sm font-medium text-white">{{ currentPerformer.name }}</p>
                <p class="text-dim mt-0.5 text-xs">From ThePornDB scene</p>
            </div>
        </div>

        <!-- Loading -->
        <div v-if="searchingActors" class="flex justify-center py-4">
            <LoadingSpinner />
        </div>

        <!-- Local Actor Search Results -->
        <div
            v-else-if="actorMatchStep === 'search' && localActorResults.length > 0"
            class="space-y-3"
        >
            <p class="text-dim text-xs">
                Found similar actors in your library. Is this the same person?
            </p>
            <div class="grid grid-cols-3 gap-2 sm:grid-cols-4 lg:grid-cols-5">
                <div
                    v-for="actor in localActorResults"
                    :key="actor.id"
                    @click="selectLocalActor(actor)"
                    class="border-border bg-surface hover:border-lava/40 cursor-pointer
                        overflow-hidden rounded-lg border transition-colors"
                >
                    <div class="bg-void aspect-[2/3] w-full">
                        <img
                            v-if="actor.image_url"
                            :src="actor.image_url"
                            :alt="actor.name"
                            class="h-full w-full object-cover"
                        />
                        <div v-else class="text-dim flex h-full w-full items-center justify-center">
                            <Icon name="heroicons:user" size="32" />
                        </div>
                    </div>
                    <div class="p-2">
                        <p class="truncate text-center text-xs font-medium text-white">
                            {{ actor.name }}
                        </p>
                    </div>
                </div>
            </div>
            <button
                @click="skipLocalMatch"
                class="text-dim w-full text-center text-xs transition-colors hover:text-white"
            >
                None of these - search ThePornDB instead
            </button>
        </div>

        <!-- PornDB Search Results -->
        <div v-else-if="actorMatchStep === 'porndb'" class="space-y-3">
            <p class="text-dim text-xs">
                {{
                    porndbPerformerResults.length > 0
                        ? 'Select a performer to create a new actor:'
                        : 'No performers found on ThePornDB.'
                }}
            </p>
            <div
                v-if="porndbPerformerResults.length > 0"
                class="grid grid-cols-3 gap-2 sm:grid-cols-4 lg:grid-cols-5"
            >
                <div
                    v-for="performer in porndbPerformerResults"
                    :key="performer.id"
                    @click="selectPornDBPerformer(performer)"
                    class="border-border bg-surface hover:border-lava/40 cursor-pointer
                        overflow-hidden rounded-lg border transition-colors"
                >
                    <div class="bg-void aspect-2/3 w-full">
                        <img
                            v-if="performer.image"
                            :src="performer.image"
                            :alt="performer.name"
                            class="h-full w-full object-cover"
                        />
                        <div v-else class="text-dim flex h-full w-full items-center justify-center">
                            <Icon name="heroicons:user" size="32" />
                        </div>
                    </div>
                    <div class="p-2">
                        <p class="truncate text-center text-xs font-medium text-white">
                            {{ performer.name }}
                        </p>
                    </div>
                </div>
            </div>
            <div class="flex justify-center gap-2">
                <button
                    v-if="currentPerformer"
                    @click="createActorWithMetadata"
                    class="border-border hover:border-lava/40 rounded-lg border px-3 py-1.5 text-xs
                        text-white transition-colors"
                >
                    Create actor "{{ currentPerformer.name }}"
                </button>
                <button
                    @click="skipPerformer"
                    class="text-dim px-3 py-1.5 text-xs transition-colors hover:text-white"
                >
                    Skip
                </button>
            </div>
        </div>

        <!-- Create option -->
        <div v-else-if="actorMatchStep === 'create'" class="space-y-3 text-center">
            <p class="text-dim text-xs">No matches found. Would you like to create a new actor?</p>
            <div class="flex justify-center gap-2">
                <button
                    v-if="currentPerformer"
                    @click="createActorWithMetadata"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs font-semibold
                        text-white transition-all"
                >
                    Create "{{ currentPerformer.name }}"
                </button>
                <button
                    @click="skipPerformer"
                    class="text-dim px-3 py-1.5 text-xs transition-colors hover:text-white"
                >
                    Skip
                </button>
            </div>
        </div>
    </div>
</template>
