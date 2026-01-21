<script setup lang="ts">
import { useVideoStore } from '~/stores/videos';

const store = useVideoStore();
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

    if (e.dataTransfer?.files.length) {
        handleFileSelect(e.dataTransfer.files[0]);
    }
};

const onFileChange = (e: Event) => {
    const target = e.target as HTMLInputElement;
    if (target.files?.length) {
        handleFileSelect(target.files[0]);
    }
};

const handleFileSelect = (file: File) => {
    selectedFile.value = file;
    // Auto-populate title if empty
    if (!title.value) {
        title.value = file.name.replace(/\.[^/.]+$/, '');
    }
};

const upload = async () => {
    if (!selectedFile.value) return;

    try {
        await store.uploadVideo(selectedFile.value, title.value);
        // Reset form
        selectedFile.value = null;
        title.value = '';
        if (fileInput.value) fileInput.value.value = '';
    } catch (e) {
        console.error(e);
    }
};
</script>

<template>
    <div class="bg-secondary/30 mb-8 rounded-2xl border border-white/5 p-6 backdrop-blur-md">
        <h2 class="mb-4 text-xl font-bold text-white">Upload Video</h2>

        <!-- Upload Form -->
        <div v-if="selectedFile" class="space-y-4">
            <div class="flex items-center gap-4 rounded-lg bg-black/20 p-4">
                <div class="flex-1 truncate text-gray-300">
                    {{ selectedFile.name }}
                </div>
                <button @click="selectedFile = null" class="text-red-400 hover:text-red-300">
                    Cancel
                </button>
            </div>

            <div>
                <label class="mb-1 block text-sm text-gray-400">Title</label>
                <input
                    v-model="title"
                    type="text"
                    class="focus:border-neon-green/50 focus:ring-neon-green/50 w-full rounded-lg
                        border border-white/10 bg-black/50 px-4 py-2 text-white focus:ring-1
                        focus:outline-none"
                    placeholder="Enter video title"
                />
            </div>

            <button
                @click="upload"
                :disabled="store.isLoading"
                class="bg-neon-green hover:bg-neon-green/90 w-full rounded-lg px-4 py-2 font-bold
                    text-black transition disabled:opacity-50"
            >
                {{ store.isLoading ? 'Uploading...' : 'Upload Video' }}
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
                `cursor-pointer rounded-xl border-2 border-dashed p-10 text-center transition-all
                duration-300`,
                isDragging
                    ? 'border-neon-green bg-neon-green/5'
                    : 'border-white/10 hover:border-white/20 hover:bg-white/5',
            ]"
        >
            <input
                ref="fileInput"
                type="file"
                class="hidden"
                accept="video/*"
                @change="onFileChange"
            />

            <div class="flex flex-col items-center gap-3">
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke-width="1.5"
                    stroke="currentColor"
                    class="h-10 w-10 text-gray-400"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5"
                    />
                </svg>
                <div class="text-lg font-medium text-white">Drop video here or click to upload</div>
                <div class="text-sm text-gray-500">MP4, MKV, AVI, WEBM</div>
            </div>
        </div>

        <!-- Error Message -->
        <div v-if="store.error" class="mt-4 rounded-lg bg-red-500/10 p-3 text-sm text-red-400">
            {{ store.error }}
        </div>
    </div>
</template>
