<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type { WatchProgress } from '~/types/homepage';
import type { SceneCardConfig } from '~/types/settings';

const props = defineProps<{
    scene: SceneListItem;
    progress?: WatchProgress;
    fluid?: boolean;
    completed?: boolean;
    rating?: number;
    liked?: boolean;
    jizzCount?: number;
    // Select mode: the checkbox is shown and a click on the card toggles the scene
    selectable?: boolean;
    // Outside select mode, the checkbox shows on hover (pointer devices only) to start a selection
    hoverSelect?: boolean;
    selected?: boolean;
    configOverride?: SceneCardConfig;
    // Card template editor preview: the card and its links do not navigate
    preview?: boolean;
}>();

const emit = defineEmits<{
    // range: shift-click, select from the last toggled scene to this one
    toggleSelection: [sceneId: number, range: boolean];
}>();

defineSlots<{
    footer?: () => unknown;
}>();

const { formatSize } = useFormatter();
const settingsStore = useSettingsStore();
const cardConfig = computed(() => props.configOverride ?? settingsStore.sceneCardConfig);

const handleCheckboxClick = (event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    emit('toggleSelection', props.scene.id, event.shiftKey);
};

// A shift-click would also extend the page's text selection
const handleCheckboxMousedown = (event: MouseEvent) => {
    if (event.shiftKey) event.preventDefault();
};

provideSceneCardPreview(!!props.preview);
const NuxtLinkComponent = resolveComponent('NuxtLink');

const handleCardClick = (event: MouseEvent) => {
    if (props.preview) {
        event.preventDefault();
        event.stopPropagation();
        return;
    }
    if (props.selectable) {
        event.preventDefault();
        event.stopPropagation();
        emit('toggleSelection', props.scene.id, event.shiftKey);
    }
};

const hovering = ref(false);
// True while the cursor is on the bottom scrub strip (preview paused, sprite frame shown)
const scrubbing = ref(false);

const handleMouseLeave = () => {
    hovering.value = false;
    scrubbing.value = false;
};

const { enabled: hoverAudio, volume: hoverAudioVolume } = useHoverAudio();

const previewVideo = ref<HTMLVideoElement | null>(null);

// The preview <video> mounts on hover. Browsers block sound until the page has had a
// click, so a rejected unmuted play() falls back to playing muted.
watch(previewVideo, (el) => {
    if (!el) return;
    el.muted = !hoverAudio.value || hoverAudioVolume.value === 0;
    el.volume = hoverAudioVolume.value / 100;
    el.play().catch(() => {
        if (el.muted || !el.isConnected) return;
        el.muted = true;
        el.play().catch(() => {});
    });
});

// Apply sound changes (header control) to the preview playing under the cursor
watch([hoverAudio, hoverAudioVolume], ([on, level]) => {
    const el = previewVideo.value;
    if (!el) return;
    el.volume = level / 100;
    el.muted = !on || level === 0;
});

watch(scrubbing, (isScrubbing) => {
    const el = previewVideo.value;
    if (!el) return;
    if (isScrubbing) {
        el.pause();
    } else {
        el.play().catch(() => {});
    }
});

const isProcessing = computed(() => isSceneProcessing(props.scene));
const isCorrupted = computed(() => isSceneCorrupted(props.scene));

const thumbnailUrl = computed(() => {
    if (!props.scene.thumbnail_path) return null;
    const base = `/thumbnails/${props.scene.id}`;
    const v = props.scene.updated_at ? new Date(props.scene.updated_at).getTime() : '';
    return v ? `${base}?v=${v}` : base;
});

const previewUrl = computed(() => {
    if (!props.scene.preview_video_path) return null;
    const v = props.scene.updated_at ? new Date(props.scene.updated_at).getTime() : '';
    return v ? `/scene-previews/${props.scene.id}?v=${v}` : `/scene-previews/${props.scene.id}`;
});

const sceneVersion = computed(() =>
    props.scene.updated_at ? String(new Date(props.scene.updated_at).getTime()) : undefined,
);

const progressPercent = computed(() => {
    if (!props.progress || props.progress.duration <= 0) return 0;
    return Math.min(100, (props.progress.last_position / props.progress.duration) * 100);
});

const hasProgress = computed(() => props.progress && progressPercent.value > 0);
</script>

<template>
    <div class="group relative" @click.capture="handleCardClick">
        <!-- Selection Checkbox -->
        <button
            v-if="selectable || hoverSelect"
            type="button"
            class="absolute top-2 left-2 z-30 flex h-5 w-5 items-center justify-center rounded
                border transition-all"
            :class="[
                selected
                    ? 'bg-lava border-lava text-white'
                    : `bg-void/60 border-white/30 text-transparent backdrop-blur-sm
                        group-hover:text-white/50 hover:border-white/50`,
                selectable
                    ? ''
                    : `pointer-events-none opacity-0 group-hover:pointer-events-auto
                        group-hover:opacity-100 focus-visible:pointer-events-auto
                        focus-visible:opacity-100`,
            ]"
            :aria-label="selected ? 'Deselect scene' : 'Select scene'"
            :aria-pressed="!!selected"
            @mousedown="handleCheckboxMousedown"
            @click="handleCheckboxClick"
        >
            <Icon name="heroicons:check" size="12" />
        </button>

        <component
            :is="preview ? 'div' : NuxtLinkComponent"
            :to="preview ? undefined : `/watch/${scene.id}`"
            class="group border-border bg-surface hover:border-border-hover hover:bg-elevated
                relative block overflow-hidden rounded-lg border transition-all duration-200"
            :class="[
                fluid ? 'w-full' : 'w-[280px] sm:w-[320px]',
                selected ? 'ring-lava/50 ring-2' : '',
            ]"
        >
            <div
                class="bg-void relative"
                :class="fluid ? 'aspect-video w-full' : 'h-[158px] sm:h-45'"
                @mouseenter="hovering = true"
                @mouseleave="handleMouseLeave"
            >
                <!-- Blurred background (stretched to fill) -->
                <img
                    v-if="thumbnailUrl"
                    :src="thumbnailUrl"
                    class="absolute inset-0 h-full w-full scale-110 object-cover blur-xl"
                    alt=""
                    aria-hidden="true"
                    loading="lazy"
                />

                <!-- Main thumbnail (maintains aspect ratio) -->
                <img
                    v-if="thumbnailUrl"
                    :src="thumbnailUrl"
                    class="absolute inset-0 z-10 h-full w-full object-contain transition-transform
                        duration-300 group-hover:scale-[1.03]"
                    :alt="scene.title"
                    loading="lazy"
                />

                <!-- Preview video on hover -->
                <video
                    v-if="hovering && previewUrl"
                    ref="previewVideo"
                    :src="previewUrl"
                    loop
                    playsinline
                    preload="auto"
                    class="absolute inset-0 z-15 h-full w-full object-contain"
                />

                <div
                    v-else-if="isProcessing"
                    class="absolute inset-0 flex items-center justify-center"
                >
                    <LoadingSpinner size="sm" />
                </div>

                <div
                    v-else-if="isCorrupted"
                    class="absolute inset-0 flex flex-col items-center justify-center gap-1"
                >
                    <Icon name="heroicons:shield-exclamation" size="24" class="text-amber-400" />
                    <span class="text-[9px] font-medium text-amber-400/80">Corrupted</span>
                </div>

                <div
                    v-else
                    class="text-dim group-hover:text-lava absolute inset-0 flex items-center
                        justify-center transition-colors"
                >
                    <Icon name="heroicons:play" size="32" />
                </div>

                <!-- Scrubber: hover the bottom strip to browse the scene -->
                <SceneCardPreviewScrubber
                    v-if="hovering && thumbnailUrl && !isProcessing"
                    :scene-id="scene.id"
                    :no-navigate="selectable"
                    :duration="scene.duration"
                    :version="sceneVersion"
                    @update:active="scrubbing = $event"
                />

                <!-- Badge zones -->
                <div
                    class="absolute top-1.5 left-1.5 z-20 transition-transform"
                    :class="selectable ? 'mt-6' : hoverSelect ? 'group-hover:translate-y-6' : ''"
                    :style="badgeZoneBoxStyle(cardConfig.badges, 'top_left')"
                >
                    <SceneCardBadgeZone
                        :zone="cardConfig.badges.top_left"
                        corner="top_left"
                        :scene="scene"
                        :liked="liked"
                        :rating="rating"
                        :jizz-count="jizzCount"
                        :completed="completed"
                    />
                </div>

                <div
                    class="absolute top-1.5 right-1.5 z-20"
                    :style="badgeZoneBoxStyle(cardConfig.badges, 'top_right')"
                >
                    <SceneCardBadgeZone
                        :zone="cardConfig.badges.top_right"
                        corner="top_right"
                        :scene="scene"
                        :liked="liked"
                        :rating="rating"
                        :jizz-count="jizzCount"
                        :completed="completed"
                    />
                </div>

                <div
                    class="absolute left-1.5 z-20"
                    :class="hasProgress ? 'bottom-3' : 'bottom-1.5'"
                    :style="badgeZoneBoxStyle(cardConfig.badges, 'bottom_left')"
                >
                    <SceneCardBadgeZone
                        :zone="cardConfig.badges.bottom_left"
                        corner="bottom_left"
                        :scene="scene"
                        :liked="liked"
                        :rating="rating"
                        :jizz-count="jizzCount"
                        :completed="completed"
                    />
                </div>

                <div
                    class="absolute right-1.5 z-20"
                    :class="hasProgress ? 'bottom-3' : 'bottom-1.5'"
                    :style="badgeZoneBoxStyle(cardConfig.badges, 'bottom_right')"
                >
                    <SceneCardBadgeZone
                        :zone="cardConfig.badges.bottom_right"
                        corner="bottom_right"
                        :scene="scene"
                        :liked="liked"
                        :rating="rating"
                        :jizz-count="jizzCount"
                        :completed="completed"
                    />
                </div>

                <!-- Watch progress bar -->
                <div v-if="hasProgress" class="absolute right-0 bottom-0 left-0 z-20 h-1">
                    <div class="h-full w-full bg-white/20">
                        <div
                            class="bg-lava h-full transition-all"
                            :style="{ width: `${progressPercent}%` }"
                        ></div>
                    </div>
                </div>

                <!-- Selected overlay -->
                <div
                    v-if="selected"
                    class="bg-lava/10 pointer-events-none absolute inset-0 z-20"
                ></div>

                <!-- Hover overlay -->
                <div
                    class="bg-lava/0 group-hover:bg-lava/5 pointer-events-none absolute inset-0 z-20
                        transition-colors duration-200"
                ></div>
            </div>

            <div class="scene-card-content p-3">
                <h3
                    class="truncate text-xs font-medium text-white/90 transition-colors
                        group-hover:text-white"
                    :title="scene.title"
                >
                    {{ scene.title }}
                </h3>

                <!-- Content rows from config -->
                <div v-if="cardConfig.content_rows.length > 0" class="mt-1.5 space-y-1">
                    <template v-for="(row, idx) in cardConfig.content_rows" :key="idx">
                        <SceneCardContentRowSplit
                            v-if="row.type === 'split' && row.left && row.right"
                            :left="row.left"
                            :right="row.right"
                            :left-mode="row.left_mode"
                            :right-mode="row.right_mode"
                            :scene="scene"
                            :rating="rating"
                            :jizz-count="jizzCount"
                        />
                        <SceneCardContentRowFull
                            v-else-if="row.type === 'full' && row.field"
                            :field="row.field"
                            :mode="row.mode"
                            :scene="scene"
                            :rating="rating"
                            :jizz-count="jizzCount"
                        />
                    </template>
                </div>

                <!-- Fallback: default footer when no content rows configured -->
                <div
                    v-else
                    class="text-dim mt-1.5 flex items-center justify-between font-mono text-[10px]"
                >
                    <slot name="footer">
                        <!-- size: absent without files:view_size -->
                        <span>{{ scene.size !== undefined ? formatSize(scene.size) : '' }}</span>
                        <UiFormattedDate :datetime="scene.created_at" />
                    </slot>
                </div>
            </div>
        </component>
    </div>
</template>
