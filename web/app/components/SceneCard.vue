<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type { WatchProgress } from '~/types/homepage';

const props = defineProps<{
    scene: SceneListItem;
    progress?: WatchProgress;
    fluid?: boolean;
    completed?: boolean;
    rating?: number;
    selectable?: boolean;
    selected?: boolean;
}>();

const emit = defineEmits<{
    toggleSelection: [sceneId: number];
}>();

const isPerfectRating = computed(() => props.rating === 5);

const slots = defineSlots<{
    footer?: () => unknown;
}>();

const { formatDuration, formatSize } = useFormatter();

const handleCheckboxClick = (event: Event) => {
    event.preventDefault();
    event.stopPropagation();
    emit('toggleSelection', props.scene.id);
};

const handleCardClick = (event: MouseEvent) => {
    if (props.selectable && (event.shiftKey || event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        emit('toggleSelection', props.scene.id);
    }
};

const isProcessing = computed(() => isSceneProcessing(props.scene));

const thumbnailUrl = computed(() => {
    if (!props.scene.thumbnail_path) return null;
    const base = `/thumbnails/${props.scene.id}`;
    const v = props.scene.updated_at ? new Date(props.scene.updated_at).getTime() : '';
    return v ? `${base}?v=${v}` : base;
});

const progressPercent = computed(() => {
    if (!props.progress || props.progress.duration <= 0) return 0;
    return Math.min(100, (props.progress.last_position / props.progress.duration) * 100);
});

const hasProgress = computed(() => props.progress && progressPercent.value > 0);
</script>

<template>
    <div class="group relative">
        <!-- Selection Checkbox -->
        <button
            v-if="selectable"
            @click="handleCheckboxClick"
            class="absolute top-2 left-2 z-30 flex h-5 w-5 items-center justify-center rounded
                border transition-all"
            :class="
                selected
                    ? 'bg-lava border-lava text-white'
                    : `bg-void/60 border-white/30 text-transparent group-hover:text-white/50
                        hover:border-white/50`
            "
        >
            <Icon name="heroicons:check" size="12" />
        </button>

        <NuxtLink
            :to="`/watch/${scene.id}`"
            @click="handleCardClick"
            class="group border-border bg-surface hover:border-border-hover hover:bg-elevated
                relative block overflow-hidden rounded-lg border transition-all duration-200"
            :class="[fluid ? 'w-full' : 'w-[320px]', selected ? 'ring-lava/50 ring-2' : '']"
        >
            <div class="bg-void relative" :class="fluid ? 'aspect-video w-full' : 'h-45'">
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

                <!-- Duration badge -->
                <div
                    v-if="scene.duration > 0"
                    class="bg-void/90 absolute right-1.5 z-20 rounded px-1.5 py-0.5 font-mono
                        text-[10px] font-medium text-white backdrop-blur-sm"
                    :class="hasProgress ? 'bottom-3' : 'bottom-1.5'"
                >
                    {{ formatDuration(scene.duration) }}
                </div>

                <!-- Completed/Watched badge -->
                <div
                    v-if="completed"
                    class="absolute top-1.5 right-1.5 z-20 rounded bg-emerald-500/90 px-1.5 py-0.5
                        text-[9px] font-semibold text-white backdrop-blur-sm"
                >
                    Watched
                </div>

                <!-- 5-star rating badge -->
                <div
                    v-if="isPerfectRating"
                    class="absolute z-20 flex h-6 w-6 items-center justify-center rounded-full
                        bg-amber-400/90 backdrop-blur-sm"
                    :class="selectable ? 'top-8 left-1.5' : 'top-1.5 left-1.5'"
                >
                    <Icon name="heroicons:star-solid" size="14" class="text-amber-900" />
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
                <div
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
