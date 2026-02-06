<script setup lang="ts">
const explorerStore = useExplorerStore();
const { getScenesMatchInfo } = useApiExplorer();

const showTagEditor = ref(false);
const showActorEditor = ref(false);
const showStudioEditor = ref(false);
const showPlaylistModal = ref(false);
const showDeleteModal = ref(false);
const showPornDBMatchModal = ref(false);

const allScenesMatched = ref(false);
const checkingMatchStatus = ref(false);

// Check if all selected scenes already have PornDB IDs
watch(
    () => explorerStore.selectedSceneIDs,
    async (ids) => {
        if (ids.size === 0) {
            allScenesMatched.value = false;
            return;
        }

        checkingMatchStatus.value = true;
        try {
            const { scenes } = await getScenesMatchInfo([...ids]);
            allScenesMatched.value = scenes.every((s) => s.porndb_scene_id !== null);
        } catch {
            allScenesMatched.value = false;
        } finally {
            checkingMatchStatus.value = false;
        }
    },
    { immediate: true },
);

const handleBulkComplete = () => {
    // Refresh folder contents after bulk operation
    explorerStore.loadFolderContents();
    explorerStore.clearSelection();
};

const handlePornDBMatchComplete = () => {
    // Refresh folder contents after bulk PornDB matching
    explorerStore.loadFolderContents();
    explorerStore.clearSelection();
};

// Expose showDeleteModal for keyboard shortcut integration
defineExpose({
    showDeleteModal,
});
</script>

<template>
    <Teleport to="body">
        <div
            class="from-void to-void/95 fixed right-0 bottom-0 left-0 z-40 border-t border-white/10
                bg-gradient-to-t px-4 py-3 backdrop-blur-lg"
        >
            <div class="mx-auto flex max-w-415 items-center justify-between">
                <!-- Selection Info -->
                <div class="flex items-center gap-3">
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="explorerStore.clearSelection()"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                    <span class="text-sm font-medium text-white">
                        {{ explorerStore.selectionCount }} selected
                    </span>
                </div>

                <!-- Actions -->
                <div class="flex items-center gap-2">
                    <button
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        @click="showTagEditor = true"
                    >
                        <Icon name="heroicons:tag" size="14" />
                        Tags
                    </button>

                    <button
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        @click="showActorEditor = true"
                    >
                        <Icon name="heroicons:user-group" size="14" />
                        Actors
                    </button>

                    <button
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        @click="showStudioEditor = true"
                    >
                        <Icon name="heroicons:building-office" size="14" />
                        Studio
                    </button>

                    <button
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        @click="showPlaylistModal = true"
                    >
                        <Icon name="heroicons:queue-list" size="14" />
                        Playlist
                    </button>

                    <div class="border-border mx-1 h-4 border-l" />

                    <!-- PornDB Match Button -->
                    <button
                        :disabled="allScenesMatched || checkingMatchStatus"
                        :title="
                            allScenesMatched
                                ? 'All selected scenes already have PornDB matches'
                                : 'Match with ThePornDB'
                        "
                        class="border-border bg-panel flex items-center gap-1.5 rounded-lg border
                            px-3 py-1.5 text-xs font-medium transition-all
                            disabled:cursor-not-allowed disabled:opacity-50"
                        :class="
                            allScenesMatched || checkingMatchStatus
                                ? 'text-dim'
                                : 'hover:border-lava/30 hover:text-lava text-white'
                        "
                        @click="showPornDBMatchModal = true"
                    >
                        <Icon
                            :name="
                                checkingMatchStatus
                                    ? 'svg-spinners:90-ring-with-bg'
                                    : 'heroicons:sparkles'
                            "
                            size="14"
                        />
                        Match
                    </button>

                    <div class="border-border mx-1 h-4 border-l" />

                    <button
                        class="border-border bg-panel flex items-center gap-1.5 rounded-lg border
                            px-3 py-1.5 text-xs font-medium text-red-400 transition-all
                            hover:border-red-500/50 hover:text-red-300"
                        @click="showDeleteModal = true"
                    >
                        <Icon name="heroicons:trash" size="14" />
                        Delete
                    </button>
                </div>
            </div>
        </div>

        <!-- Modals -->
        <ExplorerBulkTagEditor
            v-if="showTagEditor"
            :visible="showTagEditor"
            @close="showTagEditor = false"
            @complete="handleBulkComplete"
        />

        <ExplorerBulkActorEditor
            v-if="showActorEditor"
            :visible="showActorEditor"
            @close="showActorEditor = false"
            @complete="handleBulkComplete"
        />

        <ExplorerBulkStudioEditor
            v-if="showStudioEditor"
            :visible="showStudioEditor"
            @close="showStudioEditor = false"
            @complete="handleBulkComplete"
        />

        <PlaylistCreateModal
            v-if="showPlaylistModal"
            :visible="showPlaylistModal"
            :prefill-scene-ids="[...explorerStore.selectedSceneIDs]"
            @close="showPlaylistModal = false"
            @created="showPlaylistModal = false"
        />

        <ExplorerBulkDeleteConfirmModal
            v-if="showDeleteModal"
            :visible="showDeleteModal"
            @close="showDeleteModal = false"
            @complete="handleBulkComplete"
        />

        <ExplorerBulkPornDBMatchModal
            v-if="showPornDBMatchModal"
            :visible="showPornDBMatchModal"
            @close="showPornDBMatchModal = false"
            @complete="handlePornDBMatchComplete"
        />
    </Teleport>
</template>
