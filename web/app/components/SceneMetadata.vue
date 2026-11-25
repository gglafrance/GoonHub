<script setup lang="ts">
import type { Scene } from '~/types/scene';
import type { StoragePathWithCount } from '~/types/explorer';

const props = defineProps<{
    scene: Scene;
}>();

const { formatDuration, formatSize, formatBitRate, formatFrameRate } = useFormatter();
const { getStoragePaths } = useApiExplorer();

const storagePaths = ref<StoragePathWithCount[]>([]);

// File information is gated by the files:* permissions (the API omits the fields too);
// resolution and duration stay
const perms = useMyPermissions();
const canSize = computed(() => perms.has('files:view_size'));
const canPaths = computed(() => perms.has('files:view_paths'));
const canTechnical = computed(() => perms.has('files:view_technical'));
const canExplore = computed(() => perms.has('explorer:access'));
const canShare = computed(() => perms.has('scenes:share'));

const showTechnical = computed(
    () =>
        !!(props.scene.width && props.scene.height) ||
        (canTechnical.value &&
            !!(
                props.scene.frame_rate ||
                props.scene.bit_rate ||
                props.scene.video_codec ||
                props.scene.audio_codec
            )),
);
const showFile = computed(
    () => (canPaths.value && !!props.scene.original_filename) || !!props.scene.file_created_at,
);

// Fetch storage paths to resolve explorer links (explorer API)
const loadStoragePaths = async () => {
    if (!props.scene.stored_path || !canPaths.value || !canExplore.value) return;
    try {
        const response = await getStoragePaths();
        storagePaths.value = response.storage_paths;
    } catch {
        // Silently fail - path will display as plain text
    }
};

onMounted(loadStoragePaths);

const showShareModal = ref(false);

// Normalize a path: strip trailing slashes and clean up ./ prefix
const normalizePath = (p: string) => p.replace(/^\.\//, '').replace(/\/+$/, '');

// Compute clickable path segments
const pathSegments = computed(() => {
    if (!props.scene.stored_path || storagePaths.value.length === 0) return null;

    const storedPath = normalizePath(props.scene.stored_path);

    // Find matching storage path: by ID first, then by prefix
    let sp: StoragePathWithCount | undefined;
    if (props.scene.storage_path_id) {
        sp = storagePaths.value.find((s) => s.id === props.scene.storage_path_id);
    }
    if (!sp) {
        sp = storagePaths.value.find((s) => storedPath.startsWith(normalizePath(s.path)));
    }
    if (!sp) return null;

    const basePath = normalizePath(sp.path);
    if (!storedPath.startsWith(basePath)) return null;

    // Get relative path (strip base path and leading slash)
    const relativePath = storedPath.slice(basePath.length).replace(/^\//, '');
    const parts = relativePath.split('/').filter(Boolean);

    if (parts.length === 0) return null;

    // Last part is the filename
    const filename = parts[parts.length - 1];
    const folders = parts.slice(0, -1);

    return {
        storagePathId: sp.id,
        basePath: sp.path,
        folders,
        filename,
    };
});

const getExplorerLink = (folderIndex: number) => {
    if (!pathSegments.value) return '/explorer';
    const { storagePathId, folders } = pathSegments.value;
    const path = folders.slice(0, folderIndex + 1).join('/');
    return `/explorer/${storagePathId}/${path}`;
};

const getStorageRootLink = () => {
    if (!pathSegments.value) return '/explorer';
    return `/explorer/${pathSegments.value.storagePathId}`;
};

const formatResolution = (w?: number, h?: number): string => {
    if (!w || !h) return '';
    return `${w}x${h}`;
};

const formatCodec = (codec?: string): string => {
    if (!codec) return '';
    return codec.toUpperCase();
};

const getResolutionLabel = (h?: number): string => {
    if (!h) return '';
    if (h >= 4320) return '8K';
    if (h >= 2880) return '5K';
    if (h >= 2160) return '4K';
    if (h >= 1440) return 'QHD';
    if (h >= 1080) return 'FHD';
    if (h >= 720) return 'HD';
    return 'SD';
};
</script>

<template>
    <div class="sticky top-28 space-y-3">
        <!-- Info Section -->
        <div class="border-border bg-surface/30 rounded-xl border p-4 backdrop-blur-sm">
            <h1 class="text-sm leading-snug font-semibold break-all text-white">
                {{ scene.title }}
            </h1>

            <div class="mt-4 space-y-0">
                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Duration</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatDuration(scene.duration) }}
                    </span>
                </div>

                <div
                    v-if="canSize && scene.size !== undefined"
                    class="border-border flex items-center justify-between border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Size</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatSize(scene.size) }}
                    </span>
                </div>

                <div class="border-border flex items-center justify-between border-b py-2.5">
                    <span class="text-dim text-[11px]">Views</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ scene.view_count }}
                    </span>
                </div>

                <div class="flex items-center justify-between py-2.5">
                    <span class="text-dim text-[11px]">Added</span>
                    <span class="text-muted font-mono text-[11px]">
                        <UiFormattedDate :datetime="scene.created_at" />
                    </span>
                </div>
            </div>
        </div>

        <!-- Technical Section -->
        <div
            v-if="showTechnical"
            class="border-border bg-surface/30 rounded-xl border p-4 backdrop-blur-sm"
        >
            <span class="text-dim text-[10px] font-medium tracking-wider uppercase">Technical</span>
            <div class="mt-2 space-y-0">
                <div
                    v-if="scene.width && scene.height"
                    class="border-border flex items-center justify-between border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Resolution</span>
                    <span class="flex items-center gap-1.5">
                        <span class="text-muted font-mono text-[11px]">
                            {{ formatResolution(scene.width, scene.height) }}
                        </span>
                        <span
                            class="border-lava/30 bg-lava/10 text-lava rounded px-1 py-px text-[9px]
                                leading-tight font-bold"
                        >
                            {{ getResolutionLabel(scene.height) }}
                        </span>
                    </span>
                </div>

                <div
                    v-if="canTechnical && scene.frame_rate"
                    class="border-border flex items-center justify-between border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Frame Rate</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatFrameRate(scene.frame_rate) }}
                    </span>
                </div>

                <div
                    v-if="canTechnical && scene.bit_rate"
                    class="border-border flex items-center justify-between border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Bit Rate</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatBitRate(scene.bit_rate) }}
                    </span>
                </div>

                <div
                    v-if="canTechnical && scene.video_codec"
                    class="border-border flex items-center justify-between py-2.5"
                    :class="{ 'border-b': scene.audio_codec }"
                >
                    <span class="text-dim text-[11px]">Video Codec</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatCodec(scene.video_codec) }}
                    </span>
                </div>

                <div
                    v-if="canTechnical && scene.audio_codec"
                    class="flex items-center justify-between py-2.5"
                >
                    <span class="text-dim text-[11px]">Audio Codec</span>
                    <span class="text-muted font-mono text-[11px]">
                        {{ formatCodec(scene.audio_codec) }}
                    </span>
                </div>
            </div>
        </div>

        <!-- File Section -->
        <div
            v-if="showFile"
            class="border-border bg-surface/30 rounded-xl border p-4 backdrop-blur-sm"
        >
            <span class="text-dim text-[10px] font-medium tracking-wider uppercase">File</span>
            <div class="mt-2 space-y-0">
                <div
                    v-if="canPaths && scene.original_filename"
                    class="border-border border-b py-2.5"
                >
                    <span class="text-dim text-[11px]">Filename</span>
                    <p class="text-dim/70 mt-0.5 font-mono text-[10px] break-all">
                        {{ scene.original_filename }}
                    </p>
                </div>

                <div v-if="canPaths && scene.stored_path" class="border-border border-b py-2.5">
                    <span class="text-dim text-[11px]">Path</span>
                    <p
                        v-if="pathSegments"
                        class="text-dim/70 mt-0.5 font-mono text-[10px] leading-relaxed break-all"
                    >
                        <NuxtLink
                            :to="getStorageRootLink()"
                            class="text-dim/40 hover:text-lava/70 transition-colors"
                            >{{ pathSegments.basePath }}</NuxtLink
                        >
                        <span class="text-dim/30">/</span>
                        <template v-for="(folder, index) in pathSegments.folders" :key="index">
                            <NuxtLink
                                :to="getExplorerLink(index)"
                                class="text-dim/70 hover:text-lava hover:decoration-lava/50
                                    underline decoration-transparent transition-all"
                                >{{ folder }}</NuxtLink
                            >
                            <span class="text-dim/30">/</span>
                        </template>
                        <span class="text-dim/50">{{ pathSegments.filename }}</span>
                    </p>
                    <p v-else class="text-dim/70 mt-0.5 font-mono text-[10px] break-all">
                        {{ scene.stored_path }}
                    </p>
                </div>

                <WatchSceneCopies v-if="canPaths" :scene-id="scene.id" />

                <div v-if="scene.file_created_at" class="flex items-center justify-between py-2.5">
                    <span class="text-dim text-[11px]">File Date</span>
                    <span class="text-muted font-mono text-[11px]">
                        <UiFormattedDate :datetime="scene.file_created_at" />
                    </span>
                </div>
            </div>
        </div>

        <!-- Actions -->
        <div
            v-if="canShare"
            class="border-border bg-surface/30 rounded-xl border p-3 backdrop-blur-sm"
        >
            <div class="flex gap-2">
                <button
                    class="border-border bg-panel text-dim hover:border-border-hover flex-1
                        rounded-lg border py-2 text-[11px] font-medium transition-all
                        hover:text-white"
                    @click="showShareModal = true"
                >
                    <Icon name="heroicons:share" size="12" class="mr-1" />
                    Share
                </button>
            </div>
        </div>

        <ShareModal
            v-if="canShare"
            :visible="showShareModal"
            :scene-id="scene.id"
            @close="showShareModal = false"
        />
    </div>
</template>
