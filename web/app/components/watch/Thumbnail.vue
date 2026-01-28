<script setup lang="ts">
import type { Video } from '~/types/video';
import type { VttCue } from '~/composables/useVttParser';

const video = inject<Ref<Video | null>>('watchVideo');
const thumbnailVersion = inject<Ref<number>>('thumbnailVersion');
const getPlayerTime = inject<() => number>('getPlayerTime', () => 0);
const { extractThumbnail, uploadThumbnail } = useApi();
const { formatDuration } = useFormatter();

const loading = ref(false);
const error = ref<string | null>(null);
const message = ref<string | null>(null);

const uploadFile = ref<File | null>(null);
const uploadPreview = ref<string | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);

const vttCues = ref<VttCue[]>([]);
const spritesLoading = ref(false);

const currentTime = ref(0);
const timeInterval = ref<ReturnType<typeof setInterval> | null>(null);

// Calculate the full sprite sheet grid dimensions.
// All sheets have the same pixel dimensions (the last one is padded with black),
// so we use the maximum across all cues to get the complete grid size.
const sheetGridSize = computed(() => {
    let maxWidth = 0;
    let maxHeight = 0;
    for (const cue of vttCues.value) {
        maxWidth = Math.max(maxWidth, cue.x + cue.w);
        maxHeight = Math.max(maxHeight, cue.y + cue.h);
    }
    return { width: maxWidth, height: maxHeight };
});

function getSpriteStyle(cue: VttCue) {
    const grid = sheetGridSize.value;
    if (!grid.width || !grid.height) return {};
    // background-size: scale so one tile fills the container width
    const bgWidth = (grid.width / cue.w) * 100;
    const bgHeight = (grid.height / cue.h) * 100;
    // background-position as percentage
    const cols = grid.width / cue.w;
    const rows = grid.height / cue.h;
    const posX = cols > 1 ? (cue.x / cue.w / (cols - 1)) * 100 : 0;
    const posY = rows > 1 ? (cue.y / cue.h / (rows - 1)) * 100 : 0;
    return {
        aspectRatio: `${cue.w} / ${cue.h}`,
        backgroundImage: `url(${cue.url})`,
        backgroundSize: `${bgWidth}% ${bgHeight}%`,
        backgroundPosition: `${posX}% ${posY}%`,
    };
}

const currentThumbnailUrl = computed(() => {
    if (!video?.value?.thumbnail_path) return null;
    const base = `/thumbnails/${video.value.id}?size=lg`;
    const v =
        thumbnailVersion?.value ||
        (video.value.updated_at ? new Date(video.value.updated_at).getTime() : 0);
    return v ? `${base}&v=${v}` : base;
});

const formattedTime = computed(() => formatDuration(Math.floor(currentTime.value)));

onMounted(async () => {
    timeInterval.value = setInterval(() => {
        currentTime.value = getPlayerTime();
    }, 500);

    if (video?.value?.vtt_path) {
        await loadSpriteCues();
    }
});

onBeforeUnmount(() => {
    if (timeInterval.value) {
        clearInterval(timeInterval.value);
    }
    if (uploadPreview.value) {
        URL.revokeObjectURL(uploadPreview.value);
    }
});

async function loadSpriteCues() {
    if (!video?.value?.id) return;
    spritesLoading.value = true;
    try {
        const response = await fetch(`/vtt/${video.value.id}`);
        const text = await response.text();
        const cues: VttCue[] = [];

        const blocks = text.split('\n\n');
        for (const block of blocks) {
            const lines = block.trim().split('\n');
            for (let i = 0; i < lines.length; i++) {
                const line = lines[i];
                if (line && line.includes('-->')) {
                    const [startStr, endStr] = line.split('-->');
                    if (!startStr || !endStr) continue;
                    const start = parseVttTimeLocal(startStr);
                    const end = parseVttTimeLocal(endStr);
                    const urlLine = lines[i + 1]?.trim();
                    if (!urlLine) continue;

                    const hashIndex = urlLine.indexOf('#xywh=');
                    if (hashIndex === -1) continue;

                    const spriteUrl = urlLine.substring(0, hashIndex);
                    const coords = urlLine
                        .substring(hashIndex + 6)
                        .split(',')
                        .map(Number);
                    cues.push({
                        start,
                        end,
                        url: spriteUrl,
                        x: coords[0] ?? 0,
                        y: coords[1] ?? 0,
                        w: coords[2] ?? 0,
                        h: coords[3] ?? 0,
                    });
                }
            }
        }
        vttCues.value = cues;
    } catch {
        // VTT not available
    } finally {
        spritesLoading.value = false;
    }
}

function parseVttTimeLocal(timeStr: string): number {
    const parts = timeStr.trim().split(':');
    if (parts.length < 3) return 0;
    const hours = parseInt(parts[0] || '0');
    const minutes = parseInt(parts[1] || '0');
    const secParts = (parts[2] || '0').split('.');
    const seconds = parseInt(secParts[0] || '0');
    const millis = parseInt(secParts[1] || '0');
    return hours * 3600 + minutes * 60 + seconds + millis / 1000;
}

function onFileSelect(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    if (uploadPreview.value) {
        URL.revokeObjectURL(uploadPreview.value);
    }

    uploadFile.value = file;
    uploadPreview.value = URL.createObjectURL(file);
}

function clearUpload() {
    if (uploadPreview.value) {
        URL.revokeObjectURL(uploadPreview.value);
    }
    uploadFile.value = null;
    uploadPreview.value = null;
    if (fileInput.value) {
        fileInput.value.value = '';
    }
}

async function handleUpload() {
    if (!uploadFile.value || !video?.value) return;
    loading.value = true;
    error.value = null;
    message.value = null;

    try {
        await uploadThumbnail(video.value.id, uploadFile.value);
        message.value = 'Thumbnail updated from upload';
        if (thumbnailVersion) thumbnailVersion.value = Date.now();
        clearUpload();
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to upload thumbnail';
    } finally {
        loading.value = false;
    }
}

async function handleExtractFromPlayer() {
    if (!video?.value) return;
    const time = getPlayerTime();
    if (time <= 0) return;

    loading.value = true;
    error.value = null;
    message.value = null;

    try {
        await extractThumbnail(video.value.id, time);
        message.value = `Thumbnail extracted at ${formatDuration(Math.floor(time))}`;
        if (thumbnailVersion) thumbnailVersion.value = Date.now();
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to extract thumbnail';
    } finally {
        loading.value = false;
    }
}

async function handleSpriteClick(cue: VttCue) {
    if (!video?.value) return;
    loading.value = true;
    error.value = null;
    message.value = null;

    try {
        await extractThumbnail(video.value.id, cue.start);
        message.value = `Thumbnail extracted at ${formatDuration(Math.floor(cue.start))}`;
        if (thumbnailVersion) thumbnailVersion.value = Date.now();
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to extract thumbnail';
    } finally {
        loading.value = false;
    }
}
</script>

<template>
    <div class="space-y-4">
        <!-- Status messages -->
        <div
            v-if="message"
            class="flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/5
                px-3 py-2"
        >
            <Icon name="heroicons:check-circle" size="14" class="text-emerald-400" />
            <span class="text-xs text-emerald-300">{{ message }}</span>
        </div>
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-red-300">{{ error }}</span>
        </div>

        <!-- Current thumbnail preview -->
        <div v-if="currentThumbnailUrl" class="space-y-2">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">
                Current Thumbnail
            </h3>
            <div class="border-border inline-block overflow-hidden rounded-lg border">
                <img
                    :src="currentThumbnailUrl"
                    alt="Current thumbnail"
                    class="h-auto max-h-32 w-auto max-w-full object-contain"
                />
            </div>
        </div>

        <!-- Extract from player -->
        <div class="space-y-2">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">
                Extract from Player
            </h3>
            <div class="border-border bg-surface flex items-center gap-3 rounded-lg border p-3">
                <div class="text-dim flex items-center gap-2 font-mono text-xs">
                    <Icon name="heroicons:clock" size="14" class="text-lava" />
                    <span class="text-white">{{ formattedTime }}</span>
                </div>
                <button
                    @click="handleExtractFromPlayer"
                    :disabled="loading || currentTime <= 0"
                    class="bg-lava/10 text-lava border-lava/30 hover:bg-lava/20 ml-auto rounded-md
                        border px-3 py-1.5 text-[11px] font-medium transition-all
                        disabled:pointer-events-none disabled:opacity-40"
                >
                    Use This Frame
                </button>
            </div>
        </div>

        <!-- Upload custom thumbnail -->
        <div class="space-y-2">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Upload Custom</h3>
            <div class="border-border bg-surface space-y-3 rounded-lg border p-3">
                <div class="flex items-center gap-3">
                    <label
                        class="border-border bg-panel hover:border-border-hover cursor-pointer
                            rounded-md border px-3 py-1.5 text-[11px] font-medium text-white
                            transition-all"
                    >
                        Choose Image
                        <input
                            ref="fileInput"
                            type="file"
                            accept=".jpg,.jpeg,.png,.webp"
                            class="hidden"
                            @change="onFileSelect"
                        />
                    </label>
                    <span v-if="uploadFile" class="text-dim truncate text-[11px]">
                        {{ uploadFile.name }}
                    </span>
                </div>
                <div v-if="uploadPreview" class="flex items-start gap-3">
                    <div class="border-border overflow-hidden rounded-lg border">
                        <img
                            :src="uploadPreview"
                            alt="Upload preview"
                            class="h-auto max-h-24 w-auto max-w-40 object-contain"
                        />
                    </div>
                    <div class="flex gap-2">
                        <button
                            @click="handleUpload"
                            :disabled="loading"
                            class="bg-lava/10 text-lava border-lava/30 hover:bg-lava/20 rounded-md
                                border px-3 py-1.5 text-[11px] font-medium transition-all
                                disabled:pointer-events-none disabled:opacity-40"
                        >
                            Upload
                        </button>
                        <button
                            @click="clearUpload"
                            :disabled="loading"
                            class="text-dim border-border bg-panel hover:border-border-hover
                                rounded-md border px-3 py-1.5 text-[11px] font-medium transition-all
                                hover:text-white disabled:pointer-events-none disabled:opacity-40"
                        >
                            Cancel
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Sprite timecodes grid -->
        <div class="space-y-2">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">
                Select from Sprites
            </h3>
            <div v-if="spritesLoading" class="flex items-center gap-2 py-4">
                <LoadingSpinner />
            </div>
            <div
                v-else-if="vttCues.length === 0"
                class="border-border bg-surface rounded-lg border p-4 text-center"
            >
                <p class="text-dim text-xs">
                    Sprite sheets not available. Process the video first.
                </p>
            </div>
            <div v-else class="grid grid-cols-4 gap-1.5 sm:grid-cols-6 md:grid-cols-8">
                <button
                    v-for="(cue, index) in vttCues"
                    :key="index"
                    @click="handleSpriteClick(cue)"
                    :disabled="loading"
                    class="border-border hover:border-lava/50 group relative overflow-hidden rounded
                        border transition-all disabled:pointer-events-none disabled:opacity-40"
                    :title="formatDuration(Math.floor(cue.start))"
                >
                    <div class="w-full" :style="getSpriteStyle(cue)" />
                    <div
                        class="absolute inset-x-0 bottom-0 bg-black/70 px-1 py-0.5 text-center
                            font-mono text-[9px] text-white opacity-0 transition-opacity
                            group-hover:opacity-100"
                    >
                        {{ formatDuration(Math.floor(cue.start)) }}
                    </div>
                </button>
            </div>
        </div>

        <!-- Loading overlay -->
        <div
            v-if="loading"
            class="border-border bg-surface flex items-center gap-2 rounded-lg border p-3"
        >
            <LoadingSpinner />
            <span class="text-dim text-xs">Processing thumbnail...</span>
        </div>
    </div>
</template>
