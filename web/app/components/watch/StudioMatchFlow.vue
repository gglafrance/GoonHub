<script setup lang="ts">
import type { Studio, StudioListItem } from '~/types/studio';
import type { PornDBSiteDetails } from '~/types/porndb';

const props = defineProps<{
    sceneId: number;
    siteName: string;
}>();

const emit = defineEmits<{
    done: [];
    error: [message: string];
}>();

const { fetchStudios, createStudio, setSceneStudio } = useApiStudios();
const { searchPornDBSites, getPornDBSite } = useApiPornDB();

const matchStep = ref<'search' | 'porndb' | 'create' | null>('search');
const localStudioResults = ref<StudioListItem[]>([]);
const porndbSiteResults = ref<PornDBSiteDetails[]>([]);
const searching = ref(false);

async function startMatching() {
    searching.value = true;
    matchStep.value = 'search';

    try {
        // Search local studios by the site name from PornDB
        const result = await fetchStudios(1, 10, props.siteName);
        localStudioResults.value = result.data || [];

        if (localStudioResults.value.length > 0) {
            matchStep.value = 'search';
        } else {
            await searchPornDBForSite();
        }
    } catch {
        // If search fails, try PornDB
        await searchPornDBForSite();
    } finally {
        searching.value = false;
    }
}

async function searchPornDBForSite() {
    searching.value = true;
    try {
        const results = await searchPornDBSites(props.siteName);
        porndbSiteResults.value = results;
        matchStep.value = 'porndb';
    } catch {
        matchStep.value = 'create';
    } finally {
        searching.value = false;
    }
}

async function selectLocalStudio(studio: StudioListItem) {
    searching.value = true;
    try {
        await setSceneStudio(props.sceneId, studio.id);
        emit('done');
    } catch (e: unknown) {
        emit('error', e instanceof Error ? e.message : 'Failed to assign studio');
    } finally {
        searching.value = false;
    }
}

async function skipLocalMatch() {
    await searchPornDBForSite();
}

async function selectPornDBSite(site: PornDBSiteDetails) {
    searching.value = true;
    try {
        // Fetch full details to get all metadata
        const details = await getPornDBSite(site.id);
        const newStudio = await createStudio({
            name: details.name,
            short_name: details.short_name,
            url: details.url,
            description: details.description,
            rating: details.rating,
            logo: details.logo,
            favicon: details.favicon,
            poster: details.poster,
            porndb_id: details.id,
        });
        await setSceneStudio(props.sceneId, newStudio.id);
        emit('done');
    } catch {
        // If fetching details fails, try basic creation
        try {
            const newStudio = await createStudio({
                name: site.name,
                short_name: site.short_name,
                url: site.url,
                logo: site.logo,
                porndb_id: site.id,
            });
            await setSceneStudio(props.sceneId, newStudio.id);
            emit('done');
        } catch (e: unknown) {
            emit('error', e instanceof Error ? e.message : 'Failed to create studio');
        }
    } finally {
        searching.value = false;
    }
}

async function createStudioWithName() {
    searching.value = true;
    try {
        const newStudio = await createStudio({
            name: props.siteName,
        });
        await setSceneStudio(props.sceneId, newStudio.id);
        emit('done');
    } catch (e: unknown) {
        emit('error', e instanceof Error ? e.message : 'Failed to create studio');
    } finally {
        searching.value = false;
    }
}

function skip() {
    emit('done');
}

onMounted(() => {
    startMatching();
});
</script>

<template>
    <div class="space-y-4">
        <!-- Site header -->
        <div class="border-border bg-surface flex items-center gap-4 rounded-lg border p-4">
            <div
                class="bg-void text-dim flex h-14 w-14 shrink-0 items-center justify-center
                    overflow-hidden rounded-lg"
            >
                <Icon name="heroicons:building-office-2" size="24" />
            </div>
            <div>
                <p class="text-sm font-medium text-white">{{ siteName }}</p>
                <p class="text-dim mt-0.5 text-xs">From ThePornDB scene</p>
            </div>
        </div>

        <!-- Loading -->
        <div v-if="searching" class="flex justify-center py-4">
            <LoadingSpinner />
        </div>

        <!-- Local Studio Search Results -->
        <div v-else-if="matchStep === 'search' && localStudioResults.length > 0" class="space-y-3">
            <p class="text-dim text-xs">
                Found similar studios in your library. Is this the same studio?
            </p>
            <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
                <div
                    v-for="studio in localStudioResults"
                    :key="studio.id"
                    @click="selectLocalStudio(studio)"
                    class="border-border bg-surface hover:border-lava/40 cursor-pointer
                        overflow-hidden rounded-lg border transition-colors"
                >
                    <div class="bg-void flex aspect-square w-full items-center justify-center p-4">
                        <img
                            v-if="studio.logo"
                            :src="studio.logo"
                            :alt="studio.name"
                            class="max-h-full max-w-full object-contain"
                        />
                        <Icon
                            v-else
                            name="heroicons:building-office-2"
                            size="32"
                            class="text-dim"
                        />
                    </div>
                    <div class="p-2">
                        <p class="truncate text-center text-xs font-medium text-white">
                            {{ studio.name }}
                        </p>
                        <p v-if="studio.scene_count" class="text-dim text-center text-[10px]">
                            {{ studio.scene_count }} scene{{ studio.scene_count === 1 ? '' : 's' }}
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
        <div v-else-if="matchStep === 'porndb'" class="space-y-3">
            <p class="text-dim text-xs">
                {{
                    porndbSiteResults.length > 0
                        ? 'Select a site to create a new studio:'
                        : 'No sites found on ThePornDB.'
                }}
            </p>
            <div
                v-if="porndbSiteResults.length > 0"
                class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4"
            >
                <div
                    v-for="site in porndbSiteResults"
                    :key="site.id"
                    @click="selectPornDBSite(site)"
                    class="border-border bg-surface hover:border-lava/40 cursor-pointer
                        overflow-hidden rounded-lg border transition-colors"
                >
                    <div class="bg-void flex aspect-square w-full items-center justify-center p-4">
                        <img
                            v-if="site.logo"
                            :src="site.logo"
                            :alt="site.name"
                            class="max-h-full max-w-full object-contain"
                        />
                        <Icon
                            v-else
                            name="heroicons:building-office-2"
                            size="32"
                            class="text-dim"
                        />
                    </div>
                    <div class="p-2">
                        <p class="truncate text-center text-xs font-medium text-white">
                            {{ site.name }}
                        </p>
                    </div>
                </div>
            </div>
            <div class="flex justify-center gap-2">
                <button
                    @click="createStudioWithName"
                    class="border-border hover:border-lava/40 rounded-lg border px-3 py-1.5 text-xs
                        text-white transition-colors"
                >
                    Create studio "{{ siteName }}"
                </button>
                <button
                    @click="skip"
                    class="text-dim px-3 py-1.5 text-xs transition-colors hover:text-white"
                >
                    Skip
                </button>
            </div>
        </div>

        <!-- Create option -->
        <div v-else-if="matchStep === 'create'" class="space-y-3 text-center">
            <p class="text-dim text-xs">No matches found. Would you like to create a new studio?</p>
            <div class="flex justify-center gap-2">
                <button
                    @click="createStudioWithName"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs font-semibold
                        text-white transition-all"
                >
                    Create "{{ siteName }}"
                </button>
                <button
                    @click="skip"
                    class="text-dim px-3 py-1.5 text-xs transition-colors hover:text-white"
                >
                    Skip
                </button>
            </div>
        </div>
    </div>
</template>
