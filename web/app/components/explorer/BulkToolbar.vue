<script setup lang="ts">
const explorerStore = useExplorerStore();

const showTagEditor = ref(false);
const showActorEditor = ref(false);
const showStudioEditor = ref(false);
const showDeleteModal = ref(false);

const handleBulkComplete = () => {
    // Refresh folder contents after bulk operation
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
            class="fixed right-0 bottom-0 left-0 z-40 border-t border-white/10 bg-gradient-to-t
                from-void to-void/95 px-4 py-3 backdrop-blur-lg"
        >
            <div class="mx-auto flex max-w-415 items-center justify-between">
                <!-- Selection Info -->
                <div class="flex items-center gap-3">
                    <button
                        @click="explorerStore.clearSelection()"
                        class="text-dim hover:text-white transition-colors"
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
                        @click="showTagEditor = true"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                    >
                        <Icon name="heroicons:tag" size="14" />
                        Tags
                    </button>

                    <button
                        @click="showActorEditor = true"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                    >
                        <Icon name="heroicons:user-group" size="14" />
                        Actors
                    </button>

                    <button
                        @click="showStudioEditor = true"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                    >
                        <Icon name="heroicons:building-office" size="14" />
                        Studio
                    </button>

                    <div class="border-border mx-1 h-4 border-l" />

                    <button
                        @click="showDeleteModal = true"
                        class="border-border bg-panel hover:border-red-500/50 flex items-center
                            gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium text-red-400
                            transition-all hover:text-red-300"
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

        <ExplorerBulkDeleteConfirmModal
            v-if="showDeleteModal"
            :visible="showDeleteModal"
            @close="showDeleteModal = false"
            @complete="handleBulkComplete"
        />
    </Teleport>
</template>
