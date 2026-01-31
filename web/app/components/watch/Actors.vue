<script setup lang="ts">
import type { Video } from '~/types/video';
import type { Actor } from '~/types/actor';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

const video = inject<Ref<Video | null>>('watchVideo');
const { fetchActors, setVideoActors } = useApiActors();

// Inject centralized watch page data
const watchPageData = inject<WatchPageData>(WATCH_PAGE_DATA_KEY);

const error = ref<string | null>(null);

const allActors = ref<Actor[]>([]);
const allActorsLoaded = ref(false);
const loadingAllActors = ref(false);
const showActorPicker = ref(false);
const showCreateModal = ref(false);
const createActorName = ref('');

const anchorRef = ref<HTMLElement | null>(null);

// Use centralized data for loading state and video actors
const loading = computed(() => watchPageData?.loading.details ?? false);
const videoActors = computed(() => watchPageData?.actors.value ?? []);

const availableActors = computed(() =>
    allActors.value.filter((a) => !videoActors.value.some((va) => va.id === a.id)),
);

async function loadAllActors() {
    if (allActorsLoaded.value || loadingAllActors.value) return;
    loadingAllActors.value = true;

    try {
        const res = await fetchActors(1, 100);
        allActors.value = res.data || [];
        allActorsLoaded.value = true;
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load actors';
    } finally {
        loadingAllActors.value = false;
    }
}

async function onAddActorClick() {
    if (showActorPicker.value) {
        showActorPicker.value = false;
        return;
    }
    await loadAllActors();
    showActorPicker.value = true;
}

async function addActor(actorId: number) {
    if (!video?.value) return;
    error.value = null;

    const newIds = [...videoActors.value.map((a) => a.id), actorId];

    try {
        const res = await setVideoActors(video.value.id, newIds);
        // Update centralized data
        watchPageData?.setActors(res.data || []);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update actors';
    }
}

async function removeActor(actorId: number) {
    if (!video?.value) return;
    error.value = null;

    const newIds = videoActors.value.filter((a) => a.id !== actorId).map((a) => a.id);

    try {
        const res = await setVideoActors(video.value.id, newIds);
        // Update centralized data
        watchPageData?.setActors(res.data || []);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update actors';
    }
}

function onCreateRequest(name: string) {
    createActorName.value = name;
    showActorPicker.value = false;
    showCreateModal.value = true;
}

async function onActorCreated(actor: Actor) {
    showCreateModal.value = false;
    createActorName.value = '';
    // Add the new actor to the available list
    allActors.value.push(actor);
    // Automatically add to video
    await addActor(actor.id);
}
</script>

<template>
    <div class="space-y-2">
        <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Actors</h3>

        <!-- Error -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-red-300">{{ error }}</span>
        </div>

        <div v-if="loading" class="flex items-center gap-2 py-2">
            <LoadingSpinner />
        </div>

        <!-- Actor cards grid -->
        <div v-else class="flex flex-wrap gap-2">
            <!-- Applied actors as vertical cards -->
            <div v-for="actor in videoActors" :key="actor.id" class="group relative w-20">
                <NuxtLink
                    :to="`/actors/${actor.uuid}`"
                    class="border-border bg-surface hover:border-lava/40 block overflow-hidden
                        rounded-lg border transition-colors"
                >
                    <!-- Portrait image -->
                    <div class="bg-void relative aspect-[2/3] w-full">
                        <img
                            v-if="actor.image_url"
                            :src="actor.image_url"
                            :alt="actor.name"
                            class="h-full w-full object-cover"
                        />
                        <div v-else class="text-dim flex h-full w-full items-center justify-center">
                            <Icon name="heroicons:user" size="24" />
                        </div>
                    </div>
                    <!-- Name -->
                    <div class="p-1.5">
                        <p
                            class="truncate text-center text-[11px] font-medium text-white/90"
                            :title="actor.name"
                        >
                            {{ actor.name }}
                        </p>
                    </div>
                </NuxtLink>
                <!-- Remove button -->
                <button
                    @click="removeActor(actor.id)"
                    class="bg-void/80 hover:bg-lava absolute -top-1 -right-1 flex h-5 w-5
                        items-center justify-center rounded-full opacity-0 backdrop-blur-sm
                        transition-all group-hover:opacity-100"
                    title="Remove actor"
                >
                    <Icon name="heroicons:x-mark" size="12" class="text-white" />
                </button>
            </div>

            <!-- Add actor button -->
            <button
                ref="anchorRef"
                @click="onAddActorClick"
                class="border-border hover:border-lava/40 text-dim hover:text-lava flex w-20
                    flex-col items-center justify-center rounded-lg border border-dashed
                    transition-colors"
                :class="videoActors.length > 0 ? 'aspect-[2/3]' : 'h-20'"
                :disabled="loadingAllActors"
                title="Add actor"
            >
                <Icon
                    v-if="loadingAllActors"
                    name="heroicons:arrow-path"
                    size="20"
                    class="animate-spin"
                />
                <Icon v-else name="heroicons:plus" size="20" />
                <span class="mt-1 text-[10px]">Add</span>
            </button>

            <WatchActorPicker
                :visible="showActorPicker"
                :actors="availableActors"
                :anchor-el="anchorRef"
                @select="addActor"
                @close="showActorPicker = false"
                @create="onCreateRequest"
            />
        </div>

        <!-- Create Actor Modal -->
        <ActorsEditModal
            :visible="showCreateModal"
            :actor="null"
            :initial-name="createActorName"
            @close="showCreateModal = false"
            @created="onActorCreated"
        />
    </div>
</template>
