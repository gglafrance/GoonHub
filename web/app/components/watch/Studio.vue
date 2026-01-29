<script setup lang="ts">
import type { Video } from '~/types/video';
import type { Studio, StudioListItem } from '~/types/studio';

const video = inject<Ref<Video | null>>('watchVideo');
const detailsRefreshKey = inject<Ref<number>>('detailsRefreshKey');
const { fetchStudios, fetchVideoStudio, setVideoStudio } = useApiStudios();

const loading = ref(false);
const error = ref<string | null>(null);

const allStudios = ref<StudioListItem[]>([]);
const allStudiosLoaded = ref(false);
const loadingAllStudios = ref(false);
const videoStudio = ref<Studio | null>(null);
const showStudioPicker = ref(false);
const showCreateModal = ref(false);
const createStudioName = ref('');

const anchorRef = ref<HTMLElement | null>(null);

onMounted(async () => {
    await loadVideoStudio();
});

// Reload studio when metadata is applied externally (e.g. scene metadata fetch)
watch(
    () => detailsRefreshKey?.value,
    () => {
        loadVideoStudio();
    },
);

async function loadVideoStudio() {
    if (!video?.value) return;
    loading.value = true;
    error.value = null;

    try {
        const res = await fetchVideoStudio(video.value.id);
        videoStudio.value = res.data || null;
    } catch (err: unknown) {
        // 404 means no studio assigned, that's fine
        if (err instanceof Error && err.message.includes('not found')) {
            videoStudio.value = null;
        } else {
            error.value = err instanceof Error ? err.message : 'Failed to load studio';
        }
    } finally {
        loading.value = false;
    }
}

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
    if (!video?.value) return;
    error.value = null;
    showStudioPicker.value = false;

    try {
        const res = await setVideoStudio(video.value.id, studioId);
        videoStudio.value = res.data || null;
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to update studio';
    }
}

async function clearStudio() {
    if (!video?.value) return;
    error.value = null;
    showStudioPicker.value = false;

    try {
        await setVideoStudio(video.value.id, null);
        videoStudio.value = null;
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
        video_count: 0,
    });
    // Automatically assign to video
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

        <div v-else class="flex flex-wrap gap-2">
            <!-- Current studio card -->
            <div v-if="videoStudio" class="group relative">
                <NuxtLink
                    :to="`/studios/${videoStudio.uuid}`"
                    class="border-border bg-surface hover:border-lava/40 flex items-center gap-3
                        overflow-hidden rounded-lg border px-3 py-2 transition-colors"
                >
                    <!-- Logo -->
                    <div class="bg-void relative h-10 w-10 shrink-0 overflow-hidden rounded">
                        <img
                            v-if="videoStudio.logo"
                            :src="videoStudio.logo"
                            :alt="videoStudio.name"
                            class="h-full w-full object-contain p-0.5"
                        />
                        <div v-else class="text-dim flex h-full w-full items-center justify-center">
                            <Icon name="heroicons:building-office-2" size="18" />
                        </div>
                    </div>
                    <!-- Name -->
                    <div>
                        <p class="text-xs font-medium text-white/90">
                            {{ videoStudio.name }}
                        </p>
                        <p v-if="videoStudio.short_name" class="text-dim text-[10px]">
                            {{ videoStudio.short_name }}
                        </p>
                    </div>
                </NuxtLink>
                <!-- Remove/change button -->
                <button
                    ref="anchorRef"
                    @click="onChangeStudioClick"
                    class="bg-void/80 hover:bg-lava absolute -top-1 -right-1 flex h-5 w-5
                        items-center justify-center rounded-full opacity-0 backdrop-blur-sm
                        transition-all group-hover:opacity-100"
                    title="Change studio"
                >
                    <Icon name="heroicons:pencil-square" size="12" class="text-white" />
                </button>
            </div>

            <!-- Add studio button (when no studio) -->
            <button
                v-else
                ref="anchorRef"
                @click="onChangeStudioClick"
                class="border-border hover:border-lava/40 text-dim hover:text-lava flex h-14 w-28
                    flex-col items-center justify-center rounded-lg border border-dashed
                    transition-colors"
                :disabled="loadingAllStudios"
                title="Add studio"
            >
                <Icon
                    v-if="loadingAllStudios"
                    name="heroicons:arrow-path"
                    size="20"
                    class="animate-spin"
                />
                <Icon v-else name="heroicons:plus" size="20" />
                <span class="mt-1 text-[10px]">Add</span>
            </button>

            <WatchStudioPicker
                :visible="showStudioPicker"
                :studios="allStudios"
                :anchor-el="anchorRef"
                :current-studio-id="videoStudio?.id"
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
