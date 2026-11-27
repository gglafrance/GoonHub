<script setup lang="ts">
import type { SceneSizeCache } from '~/utils/selectionSize';

const props = defineProps<{
    sceneIds: number[];
    selectionCount: number;
    // Scenes loaded on the page: their sizes give the selection total without waiting
    scenes?: readonly { id: number; size?: number | null }[];
}>();

const emit = defineEmits<{
    clearSelection: [];
    complete: [];
}>();

const { getScenesMatchInfo } = useApiExplorer();
const { formatSize } = useFormatter();
const { containerClass } = useContentWidth();

const showTagEditor = ref(false);
const showActorEditor = ref(false);
const showStudioEditor = ref(false);
const showPlaylistModal = ref(false);
const showProcessModal = ref(false);
const showDeleteModal = ref(false);
const showPornDBMatchModal = ref(false);
const showTogetherModal = ref(false);

defineExpose({
    showDeleteModal,
});

const allScenesMatched = ref(false);
const checkingMatchStatus = ref(false);

// Size of each scene seen (cards of the pages browsed, then the match-info response, which
// covers selected scenes not loaded on the client, e.g. "select all" across pages)
const sizeCache: SceneSizeCache = new Map();
const sizeVersion = ref(0);
const rememberSizes = (scenes: readonly { id: number; size?: number | null }[]) => {
    if (rememberSceneSizes(sizeCache, scenes)) sizeVersion.value++;
};

watch(
    () => props.scenes,
    (scenes) => {
        if (scenes) rememberSizes(scenes);
    },
    { immediate: true },
);

// Sizes are omitted by the API without files:view_size: no total then
const perms = useMyPermissions();
const canEdit = computed(() => perms.has('scenes:edit'));
// Processing phases go through the job admin routes
const canProcess = computed(() => perms.has('jobs:manage'));
const canMatch = computed(() => perms.has('porndb:use') && perms.has('scenes:edit'));
const canDelete = computed(() => perms.has('scenes:delete'));
const canTogether = computed(() => perms.has('goon2gether:use'));
// Send to friends (messages:use): one message per video, at most messages.max_share_items
const canSend = computed(() => perms.has('messages:use'));
const showSendModal = ref(false);
const MAX_SEND = 10;
const togetherItems = computed(() => props.sceneIds.map((id) => ({ scene_id: id })));
const selectionSize = computed(() => {
    void sizeVersion.value;
    if (!perms.has('files:view_size')) return null;
    return selectionSizeLabel(
        summarizeSelectionSize(props.sceneIds, sizeCache),
        formatSize,
        checkingMatchStatus.value,
    );
});

// Only the latest request updates the state (the selection may change while one runs)
let matchInfoRequest = 0;

watch(
    () => props.sceneIds,
    async (ids) => {
        const request = ++matchInfoRequest;
        if (ids.length === 0) {
            allScenesMatched.value = false;
            checkingMatchStatus.value = false;
            return;
        }

        checkingMatchStatus.value = true;
        try {
            const { scenes } = await getScenesMatchInfo([...ids]);
            // Sizes stay valid whichever request they come from
            rememberSizes(scenes);
            if (request !== matchInfoRequest) return;
            allScenesMatched.value = scenes.every((s) => s.porndb_scene_id !== null);
        } catch {
            if (request === matchInfoRequest) allScenesMatched.value = false;
        } finally {
            if (request === matchInfoRequest) checkingMatchStatus.value = false;
        }
    },
    { immediate: true },
);

const handleBulkComplete = () => {
    emit('complete');
    emit('clearSelection');
};

const handlePornDBMatchComplete = () => {
    emit('complete');
    emit('clearSelection');
};
</script>

<template>
    <Teleport to="body">
        <div
            class="from-void to-void/95 fixed right-0 bottom-[var(--app-bottom-nav-h)] left-0 z-40
                border-t border-white/10 bg-gradient-to-t px-4 py-3 backdrop-blur-lg"
        >
            <div class="flex items-center justify-between" :class="containerClass">
                <!-- Selection Info -->
                <div class="flex items-center gap-3">
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="$emit('clearSelection')"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                    <span class="text-sm font-medium text-white">
                        {{ selectionCount }} selected
                    </span>
                    <span
                        v-if="selectionSize"
                        class="text-dim flex items-center gap-1.5 text-xs"
                        title="Total disk size of the selection"
                    >
                        <span class="text-white/20">&middot;</span>
                        <span class="font-mono text-white/80">{{ selectionSize.size }}</span>
                        <span v-if="selectionSize.note" class="text-[11px]">
                            ({{ selectionSize.note }})
                        </span>
                    </span>
                    <Icon
                        v-else-if="checkingMatchStatus"
                        name="svg-spinners:90-ring-with-bg"
                        size="12"
                        class="text-dim"
                    />
                </div>

                <!-- Actions -->
                <div class="flex items-center gap-2">
                    <button
                        v-if="canEdit"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        @click="showTagEditor = true"
                    >
                        <Icon name="heroicons:tag" size="14" />
                        Tags
                    </button>

                    <button
                        v-if="canEdit"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        @click="showActorEditor = true"
                    >
                        <Icon name="heroicons:user-group" size="14" />
                        Actors
                    </button>

                    <button
                        v-if="canEdit"
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

                    <button
                        v-if="canSend"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                        :disabled="sceneIds.length > MAX_SEND"
                        :title="
                            sceneIds.length > MAX_SEND
                                ? `Send at most ${MAX_SEND} videos at once`
                                : 'Send to friends'
                        "
                        @click="showSendModal = true"
                    >
                        <Icon name="heroicons:paper-airplane" size="14" />
                        Send
                    </button>

                    <button
                        v-if="canTogether"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        title="Watch together"
                        @click="showTogetherModal = true"
                    >
                        <Icon name="heroicons:users" size="14" />
                        Together
                    </button>

                    <button
                        v-if="canProcess"
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex
                            items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium
                            text-white transition-all"
                        @click="showProcessModal = true"
                    >
                        <Icon name="heroicons:cpu-chip" size="14" />
                        Process
                    </button>

                    <div v-if="canMatch" class="border-border mx-1 h-4 border-l" />

                    <!-- PornDB Match Button -->
                    <button
                        v-if="canMatch"
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

                    <div v-if="canDelete" class="border-border mx-1 h-4 border-l" />

                    <button
                        v-if="canDelete"
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
            :scene-ids="sceneIds"
            :selection-count="selectionCount"
            @close="showTagEditor = false"
            @complete="handleBulkComplete"
        />

        <ActorPickerModal
            v-if="showActorEditor"
            :visible="showActorEditor"
            multi-select
            show-mode-selector
            :scene-ids="sceneIds"
            :selection-count="selectionCount"
            @close="showActorEditor = false"
            @complete="handleBulkComplete"
        />

        <ExplorerBulkStudioEditor
            v-if="showStudioEditor"
            :visible="showStudioEditor"
            :scene-ids="sceneIds"
            :selection-count="selectionCount"
            @close="showStudioEditor = false"
            @complete="handleBulkComplete"
        />

        <PlaylistCreateModal
            v-if="showPlaylistModal"
            :visible="showPlaylistModal"
            :prefill-scene-ids="sceneIds"
            @close="showPlaylistModal = false"
            @created="showPlaylistModal = false"
        />

        <ShareModal
            v-if="showSendModal"
            :visible="showSendModal"
            :subject="{ kind: 'scenes', sceneIds }"
            @close="showSendModal = false"
        />

        <TogetherAddModal
            v-if="showTogetherModal"
            :visible="showTogetherModal"
            :items="togetherItems"
            @close="showTogetherModal = false"
        />

        <ExplorerBulkDeleteConfirmModal
            v-if="showDeleteModal"
            :visible="showDeleteModal"
            :scene-ids="sceneIds"
            :selection-count="selectionCount"
            @close="showDeleteModal = false"
            @complete="handleBulkComplete"
        />

        <ExplorerBulkPornDBMatchModal
            v-if="showPornDBMatchModal"
            :visible="showPornDBMatchModal"
            :scene-ids="sceneIds"
            @close="showPornDBMatchModal = false"
            @complete="handlePornDBMatchComplete"
        />

        <ExplorerBulkProcessModal
            v-if="showProcessModal"
            :visible="showProcessModal"
            :scene-ids="sceneIds"
            :selection-count="selectionCount"
            @close="showProcessModal = false"
            @complete="handleBulkComplete"
        />
    </Teleport>
</template>
