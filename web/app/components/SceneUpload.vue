<script setup lang="ts">
const authStore = useAuthStore();
const uploadStore = useUploadStore();
const fileInput = ref<HTMLInputElement | null>(null);
const isDragging = ref(false);

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

    const files = e.dataTransfer?.files;
    if (files) {
        queueFiles(files);
    }
};

const onFileChange = (e: Event) => {
    const target = e.target as HTMLInputElement;
    if (target.files) {
        queueFiles(target.files);
    }
    if (fileInput.value) fileInput.value.value = '';
};

const queueFiles = (files: FileList) => {
    for (const file of files) {
        const title = file.name.replace(/\.[^/.]+$/, '');
        uploadStore.addUpload(file, title);
    }
};
</script>

<template>
    <div class="border-border bg-surface/50 rounded-xl border p-4 backdrop-blur-sm">
        <div v-if="!authStore.isAuthenticated" class="py-6 text-center">
            <Icon name="heroicons:lock-closed" size="24" class="text-dim mx-auto mb-2" />
            <h3 class="text-muted text-xs font-medium">Authentication Required</h3>
            <p class="text-dim mt-0.5 text-[11px]">Sign in to upload scenes</p>
        </div>

        <div v-else>
            <h2 class="text-muted mb-3 text-xs font-semibold tracking-wider uppercase">Upload</h2>

            <div
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
                    multiple
                    @change="onFileChange"
                />

                <div class="flex flex-col items-center gap-2">
                    <div
                        class="border-border bg-panel flex h-8 w-8 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:arrow-up-tray" size="16" class="text-dim" />
                    </div>
                    <div class="text-muted text-xs font-medium">Drop scenes or click to upload</div>
                    <div class="text-dim font-mono text-[10px]">MP4, MKV, AVI, WEBM</div>
                </div>
            </div>

        </div>
    </div>
</template>
