<script setup lang="ts">
import type videojs from 'video.js';
import type { Marker } from '~/types/marker';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

type Player = ReturnType<typeof videojs>;

// Quick marker of the watch page (M): pauses at the current time, asks for the label, then the
// sub-labels, and saves the marker. Shift+M sets the end of that marker, Alt+M its start (or
// those of the last one started before the current time). Teleported into the video.js element (like the gesture
// overlay) so that it also works in fullscreen.
const props = defineProps<{
    player: Player | null | undefined;
    // video.js root element (null until the player is mounted)
    target: HTMLElement | null | undefined;
    sceneId: number;
}>();

const emit = defineEmits<{
    openChange: [open: boolean];
    saved: [marker: Marker];
}>();

type ToastKind = 'success' | 'error';
const toast = ref<{ id: number; message: string; kind: ToastKind } | null>(null);
let toastTimer: ReturnType<typeof setTimeout> | null = null;

const showToast = (message: string, kind: ToastKind = 'success') => {
    if (toastTimer) clearTimeout(toastTimer);
    toast.value = { id: Date.now(), message, kind };
    toastTimer = setTimeout(() => {
        toast.value = null;
        toastTimer = null;
    }, 2600);
};

// The new marker shows at once on the timeline and in the Markers tab; its thumbnail is made
// in the background
const watchPageData = inject<WatchPageData | undefined>(WATCH_PAGE_DATA_KEY, undefined);
const { insertMarker, waitForThumbnail, applyMarkerUpdate } = useNewMarkers({
    markers: watchPageData?.markers,
    refresh: watchPageData?.refreshMarkers,
});

// Last marker made with the quick marker during this visit of the scene (Shift+M, Alt+M target)
const lastQuickMarkerId = ref<number | null>(null);

const qm = useQuickMarker({
    player: () => props.player,
    sceneId: () => props.sceneId,
    onSaved: (marker, label, time) => {
        insertMarker(marker);
        waitForThumbnail(marker.id);
        lastQuickMarkerId.value = marker.id;
        showToast(quickMarkerToastMessage(time, label));
        emit('saved', marker);
    },
});
const { isOpen, step, time, label, lastLabel, chips, saving, error } = qm;

const quickRange = useQuickMarkerRange({
    player: () => props.player,
    sceneId: () => props.sceneId,
    duration: () => watchPageData?.scene.value?.duration,
    markers: () => watchPageData?.markers.value ?? [],
    lastQuickMarkerId: () => lastQuickMarkerId.value,
    insertMarker,
    applyMarkerUpdate,
    notify: showToast,
});

watch(isOpen, (open) => emit('openChange', open));

// A scene change closes the dialog (the captured time belonged to the previous scene)
watch(
    () => props.sceneId,
    () => {
        if (isOpen.value) qm.abandon();
        lastQuickMarkerId.value = null;
    },
);

// M, Shift+M and Alt+M create or edit markers: markers:create
const perms = useMyPermissions();

const onWindowKeydown = (e: KeyboardEvent) => {
    if (isOpen.value || !props.player || !props.target) return;
    if (!perms.has('markers:create')) return;
    if (isQuickMarkerEndKey(e)) {
        e.preventDefault();
        void quickRange.setEnd();
        return;
    }
    if (isQuickMarkerStartKey(e)) {
        e.preventDefault();
        void quickRange.setStart();
        return;
    }
    if (!isQuickMarkerKey(e)) return;
    // Also keeps the "m" out of the label input focused right after
    e.preventDefault();
    qm.open();
};

// Keys typed in the dialog never reach the page shortcuts (player, palette, ...)
const stopKeys = (e: KeyboardEvent) => e.stopPropagation();

// Clicks on the dialog keep the focus in its input
const keepFocus = (e: MouseEvent) => {
    const el = e.target as HTMLElement | null;
    if (el && el.tagName !== 'INPUT') e.preventDefault();
};

onMounted(() => window.addEventListener('keydown', onWindowKeydown));
onBeforeUnmount(() => {
    window.removeEventListener('keydown', onWindowKeydown);
    if (toastTimer) clearTimeout(toastTimer);
    if (isOpen.value) qm.abandon();
});
</script>

<template>
    <Teleport v-if="target" :to="target">
        <div
            v-if="isOpen"
            class="qm-backdrop"
            data-player-overlay
            @mousedown.self.prevent="qm.close()"
        >
            <div
                class="qm-panel"
                role="dialog"
                aria-modal="true"
                aria-label="Quick marker"
                @keydown="stopKeys"
                @mousedown="keepFocus"
            >
                <div class="mb-2.5 flex items-center gap-2">
                    <Icon name="heroicons:bookmark" size="13" class="text-lava" />
                    <span class="text-xs font-semibold text-white">Quick marker</span>
                    <span
                        class="border-lava/30 bg-lava/10 text-lava rounded-md border px-1.5 py-px
                            font-mono text-[10px]"
                        >{{ formatQuickMarkerTime(time) }}</span
                    >
                    <span class="text-dim ml-auto flex items-center gap-1 text-[10px]">
                        <span :class="step === 'label' ? 'text-white' : ''">1 Label</span>
                        <Icon name="heroicons:chevron-right" size="9" />
                        <span :class="step === 'tags' ? 'text-white' : ''">2 Sub-labels</span>
                    </span>
                </div>

                <div
                    v-if="error"
                    class="border-lava/30 bg-lava/5 mb-2 flex items-start gap-1.5 rounded-md border
                        px-2 py-1.5"
                >
                    <Icon
                        name="heroicons:exclamation-triangle"
                        size="12"
                        class="text-lava mt-px shrink-0"
                    />
                    <span class="text-[11px] text-red-200">{{ error }}</span>
                </div>

                <WatchQuickmarkerLabelStep
                    v-if="step === 'label'"
                    v-model="label"
                    :labels="qm.labels.value"
                    :last-label="lastLabel"
                    :locked="qm.labelLocked.value"
                    @confirm="qm.confirmLabel()"
                    @cancel="qm.close()"
                />
                <template v-else>
                    <button
                        type="button"
                        tabindex="-1"
                        class="text-muted mb-2 flex max-w-full items-center gap-1.5 text-[11px]
                            hover:text-white"
                        title="Back to the label (Esc)"
                        @click="qm.backToLabel()"
                    >
                        <Icon name="heroicons:tag" size="11" class="text-dim shrink-0" />
                        <span class="truncate font-medium text-white">{{ label }}</span>
                    </button>
                    <WatchQuickmarkerTagStep
                        :chips="chips"
                        :tags="qm.tags.value"
                        :can-create="qm.canCreateTags.value"
                        :saving="saving"
                        @add="qm.addChip"
                        @remove="qm.removeChip"
                        @remove-last="qm.removeLastChip()"
                        @submit="qm.submit()"
                        @back="qm.backToLabel()"
                    />
                </template>

                <div
                    class="text-dim mt-2.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px]"
                >
                    <span v-if="saving" class="flex items-center gap-1 text-white">
                        <Icon name="svg-spinners:ring-resize" size="10" /> Saving...
                    </span>
                    <template v-else-if="step === 'label'">
                        <span><kbd class="qm-kbd">Enter</kbd> next</span>
                        <span><kbd class="qm-kbd">Tab</kbd> complete</span>
                        <span><kbd class="qm-kbd">Esc</kbd> cancel</span>
                    </template>
                    <template v-else>
                        <span><kbd class="qm-kbd">Enter</kbd> add / save when empty</span>
                        <span><kbd class="qm-kbd">⌫</kbd> remove last</span>
                        <span><kbd class="qm-kbd">Esc</kbd> back</span>
                    </template>
                </div>
            </div>
        </div>

        <Transition name="qm-toast">
            <div
                v-if="toast"
                :key="toast.id"
                class="qm-toast"
                :class="{ 'qm-toast-error': toast.kind === 'error' }"
                :aria-live="toast.kind === 'error' ? 'assertive' : 'polite'"
            >
                <Icon
                    v-if="toast.kind === 'error'"
                    name="heroicons:exclamation-triangle"
                    size="12"
                    class="text-lava shrink-0"
                />
                <Icon v-else name="heroicons:check" size="12" class="text-emerald shrink-0" />
                <span class="truncate">{{ toast.message }}</span>
            </div>
        </Transition>
    </Teleport>
</template>

<style scoped>
.qm-backdrop {
    position: absolute;
    inset: 0;
    z-index: 40;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: clamp(8px, 7%, 64px) 12px 8px;
    background: rgba(0, 0, 0, 0.35);
    backdrop-filter: blur(2px);
    font-family: 'Inter', system-ui, sans-serif;
    text-align: left;
    cursor: default;
}

.qm-panel {
    width: min(380px, 100%);
    max-height: 100%;
    overflow-y: auto;
    padding: 12px;
    border: 1px solid rgba(255, 77, 77, 0.25);
    border-radius: 12px;
    background: rgba(10, 10, 10, 0.88);
    backdrop-filter: blur(20px) saturate(160%);
    box-shadow:
        0 12px 40px rgba(0, 0, 0, 0.6),
        0 0 24px rgba(255, 77, 77, 0.08);
}

.qm-kbd {
    margin-right: 3px;
    padding: 0 4px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.04);
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 9px;
    color: rgba(255, 255, 255, 0.6);
}

.qm-toast {
    position: absolute;
    top: 12px;
    left: 50%;
    z-index: 41;
    display: flex;
    align-items: center;
    gap: 6px;
    max-width: calc(100% - 24px);
    padding: 5px 10px;
    transform: translateX(-50%);
    border: 1px solid rgba(16, 185, 129, 0.3);
    border-radius: 999px;
    background: rgba(8, 8, 8, 0.85);
    backdrop-filter: blur(12px);
    color: #fff;
    font-family: 'Inter', system-ui, sans-serif;
    font-size: 11px;
    pointer-events: none;
}

.qm-toast-error {
    border-color: rgba(255, 77, 77, 0.35);
}

.qm-toast-enter-active,
.qm-toast-leave-active {
    transition:
        opacity 0.15s ease,
        transform 0.15s ease;
}

.qm-toast-enter-from,
.qm-toast-leave-to {
    opacity: 0;
    transform: translate(-50%, -4px);
}
</style>
