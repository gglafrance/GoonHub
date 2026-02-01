<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

const props = defineProps<{
    scene: SceneListItem;
}>();

const explorerStore = useExplorerStore();
const { formatDuration, formatSize } = useFormatter();

const isSelected = computed(() => explorerStore.isSceneSelected(props.scene.id));

const isProcessing = computed(() => isSceneProcessing(props.scene));

const thumbnailUrl = computed(() => {
    if (!props.scene.thumbnail_path) return null;
    const base = `/thumbnails/${props.scene.id}`;
    const v = props.scene.updated_at ? new Date(props.scene.updated_at).getTime() : '';
    return v ? `${base}?v=${v}` : base;
});

const handleCheckboxClick = (event: Event) => {
    event.preventDefault();
    event.stopPropagation();
    explorerStore.toggleSceneSelection(props.scene.id);
};

const handleCardClick = (event: MouseEvent) => {
    // If shift or ctrl is held, toggle selection instead of navigating
    if (event.shiftKey || event.ctrlKey || event.metaKey) {
        event.preventDefault();
        explorerStore.toggleSceneSelection(props.scene.id);
    }
};
</script>

<template>
    <div class="group relative">
        <!-- Selection Checkbox -->
        <button
            @click="handleCheckboxClick"
            class="absolute top-2 left-2 z-10 flex h-5 w-5 items-center justify-center rounded
                border transition-all"
            :class="
                isSelected
                    ? 'bg-lava border-lava text-white'
                    : `bg-void/60 border-white/30 text-transparent group-hover:text-white/50
                        hover:border-white/50`
            "
        >
            <Icon name="heroicons:check" size="12" />
        </button>

        <!-- Scene Link -->
        <NuxtLink
            :to="`/watch/${scene.id}`"
            @click="handleCardClick"
            class="border-border bg-surface hover:border-border-hover hover:bg-elevated block
                overflow-hidden rounded-lg border transition-all duration-200"
            :class="isSelected ? 'ring-lava/50 ring-2' : ''"
        >
            <div class="bg-void relative aspect-video">
                <img
                    v-if="thumbnailUrl"
                    :src="thumbnailUrl"
                    class="absolute inset-0 h-full w-full object-cover transition-transform
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
                    class="bg-void/90 absolute right-1.5 bottom-1.5 rounded px-1.5 py-0.5 font-mono
                        text-[10px] font-medium text-white backdrop-blur-sm"
                >
                    {{ formatDuration(scene.duration) }}
                </div>

                <!-- Selected overlay -->
                <div
                    v-if="isSelected"
                    class="bg-lava/10 pointer-events-none absolute inset-0"
                ></div>

                <!-- Hover overlay -->
                <div
                    class="bg-lava/0 group-hover:bg-lava/5 pointer-events-none absolute inset-0
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
                    <span>{{ formatSize(scene.size) }}</span>
                    <NuxtTime :datetime="scene.created_at" format="short" />
                </div>
            </div>
        </NuxtLink>
    </div>
</template>
