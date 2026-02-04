<script setup lang="ts">
import type { StoragePath } from '~/types/storage';

const { fetchStoragePaths, deleteStoragePath } = useApi();
const { message, error, clearMessages } = useSettingsMessage();

const loading = ref(false);
const storagePaths = ref<StoragePath[]>([]);

// Modal state
const showModal = ref(false);
const editPath = ref<StoragePath | null>(null);

const loadStoragePaths = async () => {
    loading.value = true;
    clearMessages();
    try {
        const data = await fetchStoragePaths();
        storagePaths.value = data.storage_paths;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load storage paths';
    } finally {
        loading.value = false;
    }
};

onMounted(async () => {
    await loadStoragePaths();
});

const openCreate = () => {
    editPath.value = null;
    showModal.value = true;
};

const openEdit = (path: StoragePath) => {
    editPath.value = path;
    showModal.value = true;
};

const handleSaved = () => {
    showModal.value = false;
    message.value = editPath.value
        ? 'Storage path updated successfully'
        : 'Storage path created successfully';
    loadStoragePaths();
};

const handleDelete = async (path: StoragePath) => {
    if (!confirm(`Are you sure you want to delete "${path.name}"?`)) {
        return;
    }
    clearMessages();
    try {
        await deleteStoragePath(path.id);
        message.value = 'Storage path deleted successfully';
        loadStoragePaths();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete storage path';
    }
};

const formatDate = (dateStr: string): string => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    });
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

        <!-- Storage Paths -->
        <div class="glass-panel p-5">
            <div class="mb-4 flex items-center justify-between">
                <div>
                    <h3 class="text-sm font-semibold text-white">Storage Paths</h3>
                    <p class="text-dim mt-0.5 text-[11px]">
                        Configure where video files are stored. Mount external folders via Docker,
                        then register them here.
                    </p>
                </div>
                <button
                    class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-[11px]
                        font-semibold text-white transition-all"
                    @click="openCreate"
                >
                    Add Path
                </button>
            </div>

            <!-- Storage Paths Table -->
            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>
            <div v-else-if="storagePaths.length === 0" class="text-dim py-8 text-center text-xs">
                No storage paths configured
            </div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead>
                        <tr
                            class="text-dim border-border border-b text-[11px] tracking-wider
                                uppercase"
                        >
                            <th class="pr-4 pb-2 font-medium">Name</th>
                            <th class="pr-4 pb-2 font-medium">Path</th>
                            <th class="pr-4 pb-2 font-medium">Default</th>
                            <th class="pr-4 pb-2 font-medium">Created</th>
                            <th class="pb-2 font-medium">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="path in storagePaths"
                            :key="path.id"
                            class="border-border/50 border-b last:border-0"
                        >
                            <td class="py-2.5 pr-4 text-white">{{ path.name }}</td>
                            <td class="py-2.5 pr-4">
                                <code class="text-dim bg-void/50 rounded px-1.5 py-0.5 text-[11px]">
                                    {{ path.path }}
                                </code>
                            </td>
                            <td class="py-2.5 pr-4">
                                <span
                                    v-if="path.is_default"
                                    class="bg-emerald/15 text-emerald border-emerald/30 inline-block
                                        rounded-full border px-2 py-0.5 text-[10px] font-medium"
                                >
                                    Default
                                </span>
                            </td>
                            <td class="text-dim py-2.5 pr-4">{{ formatDate(path.created_at) }}</td>
                            <td class="py-2.5">
                                <div class="flex gap-2">
                                    <button
                                        class="text-dim text-[11px] transition-colors
                                            hover:text-white"
                                        @click="openEdit(path)"
                                    >
                                        Edit
                                    </button>
                                    <button
                                        v-if="storagePaths.length > 1"
                                        class="text-lava/70 hover:text-lava text-[11px]
                                            transition-colors"
                                        @click="handleDelete(path)"
                                    >
                                        Delete
                                    </button>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <!-- Info Panel -->
        <div class="glass-panel p-5">
            <h3 class="mb-3 text-sm font-semibold text-white">About Storage Paths</h3>
            <div class="text-dim space-y-2 text-xs">
                <p>
                    Storage paths define where video files can be located. The system uses these
                    paths as reference locations - no files are copied between paths.
                </p>
                <p>
                    <strong class="text-white/80">To add external storage:</strong>
                </p>
                <ol class="list-inside list-decimal space-y-1 pl-2">
                    <li>Mount the external folder via Docker Compose volumes</li>
                    <li>
                        Click "Add Path" and enter the mounted path (e.g., /app/external/movies)
                    </li>
                    <li>The system will validate that the path exists and is accessible</li>
                    <li>Go to Jobs > Manual and click "Scan Library" to discover videos</li>
                </ol>
                <p class="text-dim/70 mt-3 italic">
                    Note: The default storage path (./data/videos) is where uploaded videos are
                    stored.
                </p>
            </div>
        </div>

        <!-- Modal -->
        <SettingsStoragePathModal
            :visible="showModal"
            :storage-path="editPath"
            @close="showModal = false"
            @saved="handleSaved"
        />
    </div>
</template>
