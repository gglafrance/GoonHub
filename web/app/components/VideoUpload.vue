<script setup lang="ts">
const store = useVideoStore();
const authStore = useAuthStore();
const fileInput = ref<HTMLInputElement | null>(null);
const isDragging = ref(false);
const title = ref('');
const selectedFile = ref<File | null>(null);

const handleDragOver = (e: DragEvent) => {
    e.preventDefault();
    isDragging.value = true;
};

const handleDragLeave = (e: DragEvent) => {
    e.preventDefault();
    isDragging.value = false;
};

const handleDrop = (e: DragEvent) => {
    e.preventDefault();
    isDragging.value = false;

    const file = e.dataTransfer?.files[0];
    if (file) {
        handleFileSelect(file);
    }
};

const onFileChange = (e: Event) => {
    const target = e.target as HTMLInputElement;
    const file = target.files?.[0];
    if (file) {
        handleFileSelect(file);
    }
};

const handleFileSelect = (file: File) => {
    selectedFile.value = file;
    if (!title.value) {
        title.value = file.name.replace(/\.[^/.]+$/, '');
    }
};

const upload = async () => {
    if (!selectedFile.value) return;

    try {
        await store.uploadVideo(selectedFile.value, title.value);
        selectedFile.value = null;
        title.value = '';
        if (fileInput.value) fileInput.value.value = '';
    } catch (e: unknown) {
        console.error(e);
    }
};
</script>

<template>
    <div class="border-border bg-surface/50 rounded-xl border p-4 backdrop-blur-sm">
        <div v-if="!authStore.isAuthenticated" class="py-6 text-center">
            <Icon name="heroicons:lock-closed" size="24" class="text-dim mx-auto mb-2" />
            <h3 class="text-muted text-xs font-medium">Authentication Required</h3>
            <p class="text-dim mt-0.5 text-[11px]">Sign in to upload videos</p>
        </div>

        <div v-else>
            <h2 class="text-muted mb-3 text-xs font-semibold tracking-wider uppercase">Upload</h2>

            <!-- Upload Form -->
            <div v-if="selectedFile" class="space-y-3">
                <div
                    class="border-border bg-panel flex items-center gap-3 rounded-lg border px-3
                        py-2"
                >
                    <Icon name="heroicons:film" size="16" class="text-lava shrink-0" />
                    <div class="text-muted flex-1 truncate text-xs">
                        {{ selectedFile.name }}
                    </div>
                    <button
                        @click="selectedFile = null"
                        class="text-dim hover:text-lava transition-colors"
                    >
                        <Icon name="heroicons:x-mark" size="14" />
                    </button>
                </div>

                <div>
                    <label
                        class="text-dim mb-1 block text-[11px] font-medium tracking-wider uppercase"
                        >Title</label
                    >
                    <input
                        v-model="title"
                        type="text"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-xs text-white
                            transition-all focus:ring-1 focus:outline-none"
                        placeholder="Video title"
                    />
                </div>

                <button
                    @click="upload"
                    :disabled="store.isLoading"
                    class="bg-lava hover:bg-lava-glow glow-lava w-full rounded-lg px-4 py-2 text-xs
                        font-semibold text-white transition-all disabled:opacity-50
                        disabled:shadow-none"
                >
                    <span v-if="store.isLoading" class="flex items-center justify-center gap-2">
                        <div
                            class="h-3 w-3 animate-spin rounded-full border-2 border-white/30
                                border-t-white"
                        ></div>
                        Uploading...
                    </span>
                    <span v-else>Upload Video</span>
                </button>
            </div>

            <!-- Drop Zone -->
            <div
                v-else
                @dragover="handleDragOver"
                @dragleave="handleDragLeave"
                @drop="handleDrop"
                @click="fileInput?.click()"
                :class="[
                    `cursor-pointer rounded-lg border border-dashed p-6 text-center transition-all
                    duration-200`,
                    isDragging
                        ? 'border-lava/50 bg-lava/5'
                        : 'border-border hover:border-border-hover hover:bg-elevated/50',
                ]"
            >
                <input
                    ref="fileInput"
                    type="file"
                    class="hidden"
                    accept="video/*"
                    @change="onFileChange"
                />

                <div class="flex flex-col items-center gap-2">
                    <div
                        class="border-border bg-panel flex h-8 w-8 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:arrow-up-tray" size="16" class="text-dim" />
                    </div>
                    <div class="text-muted text-xs font-medium">Drop video or click to upload</div>
                    <div class="text-dim font-mono text-[10px]">MP4, MKV, AVI, WEBM</div>
                </div>
            </div>

            <!-- Error Message -->
            <ErrorAlert v-if="store.error" :message="store.error" class="mt-3" />
        </div>
    </div>
</template>
