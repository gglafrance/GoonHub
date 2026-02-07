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
    selectable?: boolean;
    selected?: boolean;
    configOverride?: SceneCardConfig;
}>();

const emit = defineEmits<{
    toggleSelection: [sceneId: number];
}>();

defineSlots<{
    footer?: () => unknown;
}>();

const { formatSize } = useFormatter();
const settingsStore = useSettingsStore();
const cardConfig = computed(() => props.configOverride ?? settingsStore.sceneCardConfig);

const handleCheckboxClick = (event: Event) => {
    event.preventDefault();
    event.stopPropagation();
    emit('toggleSelection', props.scene.id);
};

const handleCardClick = (event: MouseEvent) => {
    if (props.selectable) {
        event.preventDefault();
        event.stopPropagation();
        emit('toggleSelection', props.scene.id);
    }
};

const hovering = ref(false);

const isProcessing = computed(() => isSceneProcessing(props.scene));

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
            v-if="selectable"
            class="absolute top-2 left-2 z-30 flex h-5 w-5 items-center justify-center rounded
                border transition-all"
            :class="
                selected
                    ? 'bg-lava border-lava text-white'
                    : `bg-void/60 border-white/30 text-transparent group-hover:text-white/50
                        hover:border-white/50`
            "
            @click="handleCheckboxClick"
        >
            <Icon name="heroicons:check" size="12" />
        </button>

        <NuxtLink
            :to="`/watch/${scene.id}`"
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
                @mouseleave="hovering = false"
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
                    :src="previewUrl"
                    muted
                    loop
                    autoplay
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
                    v-else
                    class="text-dim group-hover:text-lava absolute inset-0 flex items-center
                        justify-center transition-colors"
                >
                    <Icon name="heroicons:play" size="32" />
                </div>

                <!-- Badge zones -->
                <div class="absolute top-1.5 left-1.5 z-20" :class="selectable ? 'mt-6' : ''">
                    <SceneCardBadgeZone
                        :items="cardConfig.badges.top_left.items"
                        :direction="cardConfig.badges.top_left.direction"
                        :scene="scene"
                        :liked="liked"
                        :rating="rating"
                        :jizz-count="jizzCount"
                        :completed="completed"
                    />
                </div>

                <div class="absolute top-1.5 right-1.5 z-20">
                    <SceneCardBadgeZone
                        :items="cardConfig.badges.top_right.items"
                        :direction="cardConfig.badges.top_right.direction"
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
                >
                    <SceneCardBadgeZone
                        :items="cardConfig.badges.bottom_left.items"
                        :direction="cardConfig.badges.bottom_left.direction"
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
                >
                    <SceneCardBadgeZone
                        :items="cardConfig.badges.bottom_right.items"
                        :direction="cardConfig.badges.bottom_right.direction"
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

            <div class="p-3">
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
                        <span>{{ formatSize(scene.size) }}</span>
                        <NuxtTime :datetime="scene.created_at" format="short" />
                    </slot>
                </div>
            </div>
        </NuxtLink>
    </div>
</template>
