<script setup lang="ts">
const uploadStore = useUploadStore();
const expanded = ref(false);

const completedCount = computed(
    () => uploadStore.uploads.filter((u) => u.status === 'completed').length,
);
const failedCount = computed(
    () => uploadStore.uploads.filter((u) => u.status === 'failed').length,
);

function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="uploadStore.uploads.length > 0"
            class="fixed right-4 bottom-4 z-50"
        >
            <!-- Collapsed Pill -->
            <button
                v-if="!expanded"
                @click="expanded = true"
                class="border-border/50 bg-panel/95 flex items-center gap-2 rounded-full border
                    px-3 py-2 shadow-lg backdrop-blur-md transition-all hover:border-white/20"
            >
                <div
                    v-if="uploadStore.hasActive"
                    class="border-lava/30 border-t-lava h-3.5 w-3.5 animate-spin rounded-full
                        border-2"
                ></div>
                <Icon
                    v-else-if="failedCount > 0"
                    name="heroicons:exclamation-circle"
                    size="14"
                    class="text-red-400"
                />
                <Icon
                    v-else
                    name="heroicons:check-circle"
                    size="14"
                    class="text-emerald-400"
                />
                <span class="text-muted text-xs font-medium">
                    {{ uploadStore.activeCount }} uploading
                    <span v-if="completedCount > 0" class="text-emerald-400">
                        / {{ completedCount }} done
                    </span>
                </span>
            </button>

            <!-- Expanded Panel -->
            <div
                v-else
                class="border-border/50 bg-panel/95 w-80 rounded-xl border shadow-2xl
                    backdrop-blur-md"
            >
                <!-- Header -->
                <div
                    class="border-border/30 flex items-center justify-between border-b px-3 py-2"
                >
                    <span class="text-muted text-xs font-semibold tracking-wider uppercase"
                        >Uploads</span
                    >
                    <div class="flex items-center gap-1">
                        <button
                            v-if="completedCount > 0"
                            @click="uploadStore.removeCompleted()"
                            class="text-dim hover:text-muted rounded p-1 text-[10px]
                                transition-colors"
                            title="Clear completed"
                        >
                            Clear
                        </button>
                        <button
                            @click="expanded = false"
                            class="text-dim hover:text-muted rounded p-1 transition-colors"
                        >
                            <Icon name="heroicons:chevron-down" size="14" />
                        </button>
                    </div>
                </div>

                <!-- Upload List -->
                <div class="max-h-64 overflow-y-auto p-2">
                    <div
                        v-for="upload in uploadStore.uploads"
                        :key="upload.id"
                        class="border-border/20 mb-1.5 rounded-lg border p-2 last:mb-0"
                    >
                        <div class="mb-1 flex items-center gap-2">
                            <!-- Status Icon -->
                            <div
                                v-if="upload.status === 'uploading'"
                                class="border-lava/30 border-t-lava h-3 w-3 shrink-0
                                    animate-spin rounded-full border-2"
                            ></div>
                            <Icon
                                v-else-if="upload.status === 'queued'"
                                name="heroicons:clock"
                                size="12"
                                class="text-dim shrink-0"
                            />
                            <Icon
                                v-else-if="upload.status === 'completed'"
                                name="heroicons:check-circle"
                                size="12"
                                class="shrink-0 text-emerald-400"
                            />
                            <Icon
                                v-else-if="upload.status === 'failed'"
                                name="heroicons:x-circle"
                                size="12"
                                class="shrink-0 text-red-400"
                            />

                            <!-- Title -->
                            <span class="text-muted flex-1 truncate text-[11px]">
                                {{ upload.title || upload.file.name }}
                            </span>

                            <!-- Size -->
                            <span class="text-dim text-[10px]">
                                {{ formatSize(upload.file.size) }}
                            </span>

                            <!-- Cancel Button -->
                            <button
                                v-if="upload.status === 'uploading' || upload.status === 'queued'"
                                @click="uploadStore.cancelUpload(upload.id)"
                                class="text-dim hover:text-lava shrink-0 transition-colors"
                            >
                                <Icon name="heroicons:x-mark" size="12" />
                            </button>
                            <button
                                v-else
                                @click="uploadStore.removeUpload(upload.id)"
                                class="text-dim hover:text-muted shrink-0 transition-colors"
                            >
                                <Icon name="heroicons:x-mark" size="12" />
                            </button>
                        </div>

                        <!-- Progress Bar -->
                        <div
                            v-if="upload.status === 'uploading'"
                            class="bg-void/60 h-1 overflow-hidden rounded-full"
                        >
                            <div
                                class="bg-lava h-full rounded-full transition-all duration-300"
                                :style="{ width: `${upload.progress}%` }"
                            ></div>
                        </div>

                        <!-- Error Message -->
                        <div
                            v-if="upload.status === 'failed' && upload.error"
                            class="mt-0.5 text-[10px] text-red-400/80"
                        >
                            {{ upload.error }}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </Teleport>
</template>
