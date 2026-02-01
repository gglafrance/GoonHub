<script setup lang="ts">
import type { Scene } from '~/types/scene';
import type { Studio, StudioListItem } from '~/types/studio';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

const scene = inject<Ref<Scene | null>>('watchScene');
const { fetchStudios, setSceneStudio } = useApiStudios();

// Inject centralized watch page data
const watchPageData = inject<WatchPageData>(WATCH_PAGE_DATA_KEY);

const error = ref<string | null>(null);

const allStudios = ref<StudioListItem[]>([]);
const allStudiosLoaded = ref(false);
const loadingAllStudios = ref(false);
const showStudioPicker = ref(false);
const showCreateModal = ref(false);
const createStudioName = ref('');

const anchorRef = ref<HTMLElement | null>(null);

// Use centralized data for loading state and scene studio
const loading = computed(() => watchPageData?.loading.details ?? false);
const sceneStudio = computed(() => watchPageData?.studio.value ?? null);

async function loadAllStudios() {
    if (allStudiosLoaded.value || loadingAllStudios.value) return;
    loadingAllStudios.value = true;

    try {
        const res = await fetchStudios(1, 100);
        allStudios.value = res.data || [];
        allStudiosLoaded.value = true;
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load studios';
    } finally {
        loadingAllStudios.value = false;
    }
}

async function onChangeStudioClick() {
    if (showStudioPicker.value) {
        showStudioPicker.value = false;
        return;
    }
    await loadAllStudios();
    showStudioPicker.value = true;
}

async function selectStudio(studioId: number) {
    if (!scene?.value) return;
    error.value = null;
    showStudioPicker.value = false;

    try {
        const res = await setSceneStudio(scene.value.id, studioId);
        // Update centralized data
        watchPageData?.setStudio(res.data || null);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update studio';
    }
}

async function clearStudio() {
    if (!scene?.value) return;
    error.value = null;
    showStudioPicker.value = false;

    try {
        await setSceneStudio(scene.value.id, null);
        // Update centralized data
        watchPageData?.setStudio(null);
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to remove studio';
    }
}

function onCreateRequest(name: string) {
    createStudioName.value = name;
    showStudioPicker.value = false;
    showCreateModal.value = true;
}

async function onStudioCreated(studio: Studio) {
    showCreateModal.value = false;
    createStudioName.value = '';
    // Add the new studio to the available list
    allStudios.value.push({
        id: studio.id,
        uuid: studio.uuid,
        name: studio.name,
        short_name: studio.short_name || '',
        logo: studio.logo || '',
        scene_count: 0,
    });
    // Automatically assign to scene
    await selectStudio(studio.id);
}
</script>

<template>
    <div class="space-y-2">
        <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Studio</h3>

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

        <div v-else>
            <!-- Current studio card -->
            <div v-if="sceneStudio" class="group relative inline-flex">
                <NuxtLink
                    :to="`/studios/${sceneStudio.uuid}`"
                    class="hover:border-lava/40 flex items-center gap-2 rounded-md px-1 py-0.5
                        transition-colors hover:bg-white/3"
                >
                    <!-- Logo -->
                    <div class="bg-void relative h-8 w-8 shrink-0 overflow-hidden rounded">
                        <img
                            v-if="sceneStudio.logo"
                            :src="sceneStudio.logo"
                            :alt="sceneStudio.name"
                            class="h-full w-full object-contain p-0.5"
                        />
                        <div v-else class="text-dim flex h-full w-full items-center justify-center">
                            <Icon name="heroicons:building-office-2" size="14" />
                        </div>
                    </div>
                    <!-- Name -->
                    <div class="min-w-0">
                        <p class="truncate text-sm font-medium text-white/90">
                            {{ sceneStudio.name }}
                        </p>
                        <p v-if="sceneStudio.short_name" class="text-dim truncate text-[10px]">
                            {{ sceneStudio.short_name }}
                        </p>
                    </div>
                </NuxtLink>
                <!-- Remove/change button -->
                <button
                    ref="anchorRef"
                    @click="onChangeStudioClick"
                    class="bg-void/80 hover:bg-lava absolute -top-1 -right-1 flex h-4 w-4
                        items-center justify-center rounded-full opacity-0 backdrop-blur-sm
                        transition-all group-hover:opacity-100"
                    title="Change studio"
                >
                    <Icon name="heroicons:pencil-square" size="10" class="text-white" />
                </button>
            </div>

            <!-- Add studio button (when no studio) -->
            <button
                v-else
                ref="anchorRef"
                @click="onChangeStudioClick"
                class="hover:border-lava/40 text-dim hover:text-lava -mx-1 flex items-center gap-2
                    rounded-md border border-transparent px-1 py-1 text-sm transition-colors
                    hover:bg-white/3"
                :disabled="loadingAllStudios"
                title="Add studio"
            >
                <Icon
                    v-if="loadingAllStudios"
                    name="heroicons:arrow-path"
                    size="14"
                    class="animate-spin"
                />
                <Icon v-else name="heroicons:plus-circle" size="14" />
                <span>Add studio</span>
            </button>

            <WatchStudioPicker
                :visible="showStudioPicker"
                :studios="allStudios"
                :anchor-el="anchorRef"
                :current-studio-id="sceneStudio?.id"
                @select="selectStudio"
                @clear="clearStudio"
                @close="showStudioPicker = false"
                @create="onCreateRequest"
            />
        </div>

        <!-- Create Studio Modal -->
        <StudiosEditModal
            :visible="showCreateModal"
            :studio="null"
            :initial-name="createStudioName"
            @close="showCreateModal = false"
            @created="onStudioCreated"
        />
    </div>
</template>
