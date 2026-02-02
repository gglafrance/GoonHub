<script setup lang="ts">
interface TrashedScene {
    id: number;
    title: string;
    thumbnail_path: string;
    trashed_at: string;
    expires_at: string;
}

const { listTrash, restoreScene, permanentDeleteScene, emptyTrash } = useApiAdmin();
const { message, error, clearMessages } = useSettingsMessage();

const loading = ref(false);
const trashedScenes = ref<TrashedScene[]>([]);
const total = ref(0);
const page = ref(1);
const limit = ref(20);
const retentionDays = ref(7);

// Empty trash modal state
const showEmptyModal = ref(false);
const emptyingTrash = ref(false);

const loadTrash = async () => {
    loading.value = true;
    clearMessages();
    failedThumbnails.value.clear();
    try {
        const data = await listTrash(page.value, limit.value);
        trashedScenes.value = data.data;
        total.value = data.total;
        retentionDays.value = data.retention_days;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load trash';
    } finally {
        loading.value = false;
    }
};

onMounted(async () => {
    await loadTrash();
});

const handleRestore = async (scene: TrashedScene) => {
    clearMessages();
    try {
        await restoreScene(scene.id);
        message.value = `"${scene.title}" restored successfully`;
        await loadTrash();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to restore scene';
    }
};

const handlePermanentDelete = async (scene: TrashedScene) => {
    if (!confirm(`Permanently delete "${scene.title}"? This cannot be undone.`)) {
        return;
    }
    clearMessages();
    try {
        await permanentDeleteScene(scene.id);
        message.value = `"${scene.title}" permanently deleted`;
        await loadTrash();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete scene';
    }
};

const handleEmptyTrash = async () => {
    emptyingTrash.value = true;
    clearMessages();
    try {
        const result = await emptyTrash();
        message.value = `${result.deleted} scenes permanently deleted`;
        showEmptyModal.value = false;
        await loadTrash();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to empty trash';
    } finally {
        emptyingTrash.value = false;
    }
};

const formatRelativeTime = (dateStr: string): string => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = date.getTime() - now.getTime();
    const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays <= 0) return 'Expiring soon';
    if (diffDays === 1) return '1 day left';
    return `${diffDays} days left`;
};

const formatDate = (dateStr: string): string => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    });
};

// Track scenes with failed thumbnails to show placeholder
const failedThumbnails = ref<Set<number>>(new Set());

const handleThumbnailError = (sceneId: number) => {
    failedThumbnails.value.add(sceneId);
};

const shouldShowThumbnail = (scene: TrashedScene): boolean => {
    return !!scene.thumbnail_path && !failedThumbnails.value.has(scene.id);
};
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <!-- Trash -->
        <div class="glass-panel p-5">
            <div class="mb-4 flex items-center justify-between">
                <div>
                    <h3 class="text-sm font-semibold text-white">Trash</h3>
                    <p class="text-dim mt-0.5 text-[11px]">
                        Deleted scenes are kept for {{ retentionDays }} days before permanent
                        deletion.
                    </p>
                </div>
                <button
                    v-if="trashedScenes.length > 0"
                    @click="showEmptyModal = true"
                    class="rounded-lg bg-red-600 px-3 py-1.5 text-[11px] font-semibold text-white
                        transition-all hover:bg-red-500"
                >
                    Empty Trash
                </button>
            </div>

            <!-- Loading -->
            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

            <!-- Empty state -->
            <div v-else-if="trashedScenes.length === 0" class="py-8 text-center">
                <Icon name="heroicons:trash" size="32" class="text-dim/50 mx-auto mb-2" />
                <p class="text-dim text-xs">Trash is empty</p>
            </div>

            <!-- Trashed scenes list -->
            <div v-else class="space-y-2">
                <div
                    v-for="scene in trashedScenes"
                    :key="scene.id"
                    class="border-border/50 bg-surface/30 flex items-center gap-4 rounded-lg border
                        p-3"
                >
                    <!-- Thumbnail -->
                    <div class="relative h-14 w-24 shrink-0 overflow-hidden rounded bg-black">
                        <template v-if="shouldShowThumbnail(scene)">
                            <!-- Blurred background (fills container) -->
                            <img
                                :src="`/thumbnails/${scene.id}`"
                                class="absolute inset-0 h-full w-full scale-110 object-cover
                                    blur-md"
                                alt=""
                                aria-hidden="true"
                            />
                            <!-- Main thumbnail (maintains aspect ratio) -->
                            <img
                                :src="`/thumbnails/${scene.id}`"
                                :alt="scene.title"
                                class="absolute inset-0 h-full w-full object-contain"
                                @error="handleThumbnailError(scene.id)"
                            />
                        </template>
                        <div v-else class="flex h-full items-center justify-center">
                            <Icon name="heroicons:film" size="20" class="text-dim/50" />
                        </div>
                    </div>

                    <!-- Info -->
                    <div class="min-w-0 flex-1">
                        <h4 class="truncate text-sm font-medium text-white">{{ scene.title }}</h4>
                        <p class="text-dim mt-0.5 text-[11px]">
                            Deleted: {{ formatDate(scene.trashed_at) }}
                        </p>
                    </div>

                    <!-- Expiry badge -->
                    <div
                        class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium"
                        :class="
                            formatRelativeTime(scene.expires_at) === 'Expiring soon'
                                ? 'border border-amber-500/30 bg-amber-500/15 text-amber-500'
                                : 'bg-surface/50 text-dim border-border/50 border'
                        "
                    >
                        {{ formatRelativeTime(scene.expires_at) }}
                    </div>

                    <!-- Actions -->
                    <div class="flex shrink-0 gap-2">
                        <button
                            @click="handleRestore(scene)"
                            class="text-dim hover:text-emerald rounded p-1.5 transition-colors"
                            title="Restore"
                        >
                            <Icon name="heroicons:arrow-uturn-left" size="14" />
                        </button>
                        <button
                            @click="handlePermanentDelete(scene)"
                            class="text-dim hover:text-lava rounded p-1.5 transition-colors"
                            title="Delete permanently"
                        >
                            <Icon name="heroicons:trash" size="14" />
                        </button>
                    </div>
                </div>
            </div>

            <!-- Pagination info -->
            <div v-if="total > limit" class="text-dim mt-4 text-center text-[11px]">
                Showing {{ trashedScenes.length }} of {{ total }} scenes
            </div>
        </div>
    </div>

    <!-- Empty Trash Confirmation Modal -->
    <SettingsTrashEmptyModal
        :visible="showEmptyModal"
        :count="total"
        :loading="emptyingTrash"
        @close="showEmptyModal = false"
        @confirm="handleEmptyTrash"
    />
</template>
