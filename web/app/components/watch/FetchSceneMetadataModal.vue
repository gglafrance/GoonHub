<script setup lang="ts">
import type { Scene } from '~/types/scene';
import type { PornDBScene } from '~/types/porndb';

const props = defineProps<{
    visible: boolean;
    scene: Scene | null;
}>();

const emit = defineEmits<{
    close: [];
    applied: [];
}>();

const api = useApi();

// Phase management
const phase = ref<'search' | 'preview' | 'applying'>('search');
const selectedScene = ref<PornDBScene | null>(null);

// Applying state
const applyingBasic = ref(false);
const applyError = ref('');
const performersToMatch = ref<PornDBScene['performers']>([]);
const siteToMatch = ref<string | null>(null);
const shouldMatchStudio = ref(false);

// Phase title
const phaseTitle = computed(() => {
    if (phase.value === 'search') return 'Search Scene Metadata';
    if (phase.value === 'preview') return 'Preview Metadata';
    return 'Applying Metadata';
});

// Reset when modal opens
watch(
    () => props.visible,
    (visible) => {
        if (visible) {
            phase.value = 'search';
            selectedScene.value = null;
            applyingBasic.value = false;
            applyError.value = '';
            performersToMatch.value = [];
            siteToMatch.value = null;
            shouldMatchStudio.value = false;
        }
    },
);

function onSceneSelected(scene: PornDBScene) {
    selectedScene.value = scene;
    phase.value = 'preview';
}

function goBackToSearch() {
    phase.value = 'search';
    selectedScene.value = null;
}

async function onApply(fields: {
    title: boolean;
    description: boolean;
    studio: boolean;
    thumbnail: boolean;
    performers: boolean;
    tags: boolean;
    release_date: boolean;
    markers: boolean;
}) {
    if (!props.scene || !selectedScene.value) return;

    phase.value = 'applying';
    applyingBasic.value = true;
    applyError.value = '';

    try {
        // Build the update payload (NOT including studio - that uses matching flow)
        const payload: {
            title?: string;
            description?: string;
            thumbnail_url?: string;
            tag_names?: string[];
            release_date?: string;
            porndb_scene_id?: string;
        } = {};

        if (fields.title && selectedScene.value.title) {
            payload.title = selectedScene.value.title;
        }
        if (fields.description && selectedScene.value.description) {
            payload.description = selectedScene.value.description;
        }
        if (fields.thumbnail) {
            const thumbnailUrl = selectedScene.value.image || selectedScene.value.poster;
            if (thumbnailUrl) {
                payload.thumbnail_url = thumbnailUrl;
            }
        }
        if (fields.tags && selectedScene.value.tags?.length) {
            payload.tag_names = selectedScene.value.tags.map((t: { name: string }) => t.name);
        }
        if (fields.release_date && selectedScene.value.date) {
            payload.release_date = selectedScene.value.date;
        }
        // Always include PornDB scene ID when applying metadata
        payload.porndb_scene_id = selectedScene.value.id;

        // Apply basic metadata
        if (Object.keys(payload).length > 0) {
            await api.applySceneMetadata(props.scene.id, payload);
        }

        // Import markers if selected
        if (fields.markers && selectedScene.value.markers?.length) {
            const { createMarker } = useApiMarkers();
            for (const marker of selectedScene.value.markers) {
                try {
                    await createMarker(props.scene.id, {
                        timestamp: marker.start_time,
                        label: marker.title,
                    });
                } catch {
                    // Log but don't fail entire import
                    console.warn(`Failed to import marker: ${marker.title}`);
                }
            }
        }

        applyingBasic.value = false;

        // Remember if we need to match studio (after performers)
        if (fields.studio && selectedScene.value.site?.name) {
            shouldMatchStudio.value = true;
            siteToMatch.value = selectedScene.value.site.name;
        }

        // Handle performers first if selected
        if (fields.performers && selectedScene.value.performers?.length) {
            performersToMatch.value = [...selectedScene.value.performers];
        } else if (shouldMatchStudio.value && siteToMatch.value) {
            // No performers to match, go directly to studio matching
            // (siteToMatch is already set, flow will render)
        } else {
            emit('applied');
            emit('close');
        }
    } catch (e: unknown) {
        applyError.value = e instanceof Error ? e.message : 'Failed to apply metadata';
        applyingBasic.value = false;
    }
}

function onActorMatchDone() {
    // After performers, check if we need to match studio
    performersToMatch.value = [];
    if (shouldMatchStudio.value && siteToMatch.value) {
        // Studio matching will now render
    } else {
        emit('applied');
        emit('close');
    }
}

function onActorMatchError(message: string) {
    applyError.value = message;
}

function onStudioMatchDone() {
    emit('applied');
    emit('close');
}

function onStudioMatchError(message: string) {
    applyError.value = message;
}

function handleClose() {
    emit('close');
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70
                p-4 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div
                class="glass-panel border-border my-4 flex w-full max-w-5xl flex-col border p-6"
                style="max-height: calc(100vh - 2rem)"
            >
                <!-- Header -->
                <div class="mb-4 flex shrink-0 items-center justify-between">
                    <h3 class="text-sm font-semibold text-white">
                        {{ phaseTitle }}
                    </h3>
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="handleClose"
                    >
                        <Icon name="heroicons:x-mark" size="20" />
                    </button>
                </div>

                <!-- Search Phase (v-show to preserve state when switching to preview) -->
                <WatchSceneSearch
                    v-show="phase === 'search'"
                    :scene="scene"
                    class="min-h-0 flex-1"
                    @select="onSceneSelected"
                />

                <!-- Preview Phase -->
                <WatchScenePreview
                    v-if="phase === 'preview' && selectedScene"
                    :scene="scene"
                    :porndb-scene="selectedScene"
                    class="min-h-0 flex-1 overflow-y-auto"
                    @back="goBackToSearch"
                    @apply="onApply"
                    @close="handleClose"
                />

                <!-- Applying Phase -->
                <div v-if="phase === 'applying'" class="space-y-4">
                    <!-- Error -->
                    <div
                        v-if="applyError"
                        class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2
                            text-xs"
                    >
                        {{ applyError }}
                    </div>

                    <!-- Loading basic metadata -->
                    <div v-if="applyingBasic" class="flex flex-col items-center gap-3 py-8">
                        <LoadingSpinner />
                        <p class="text-dim text-sm">Applying metadata...</p>
                    </div>

                    <!-- Actor matching sub-flow -->
                    <WatchActorMatchFlow
                        v-else-if="performersToMatch && performersToMatch.length > 0 && scene"
                        :scene-id="scene.id"
                        :performers="performersToMatch"
                        @done="onActorMatchDone"
                        @error="onActorMatchError"
                    />

                    <!-- Studio matching sub-flow (after performers are done) -->
                    <WatchStudioMatchFlow
                        v-else-if="
                            shouldMatchStudio &&
                            siteToMatch &&
                            performersToMatch &&
                            performersToMatch.length === 0 &&
                            scene
                        "
                        :scene-id="scene.id"
                        :site-name="siteToMatch"
                        @done="onStudioMatchDone"
                        @error="onStudioMatchError"
                    />
                </div>
            </div>
        </div>
    </Teleport>
</template>
