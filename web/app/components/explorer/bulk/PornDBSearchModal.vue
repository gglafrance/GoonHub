<script setup lang="ts">
import type { PornDBScene } from '~/types/porndb';
import type { SceneMatchInfo } from '~/types/explorer';

const props = defineProps<{
    visible: boolean;
    scene: SceneMatchInfo;
}>();

const emit = defineEmits<{
    close: [];
    select: [scene: PornDBScene];
}>();

// Convert SceneMatchInfo to full object for WatchSceneSearch (enables confidence calculation)
const sceneForSearch = computed(() => ({
    title: props.scene.title,
    studio: props.scene.studio,
    original_filename: props.scene.original_filename,
    actors: props.scene.actors,
    duration: props.scene.duration,
}));

function onSceneSelected(scene: PornDBScene) {
    emit('select', scene);
    emit('close');
}

// Track mousedown origin to prevent text selection from closing modal
const backdropMouseDown = ref(false);
function onBackdropMouseDown(e: MouseEvent) {
    backdropMouseDown.value = e.target === e.currentTarget;
}
function onBackdropMouseUp(e: MouseEvent) {
    if (backdropMouseDown.value && e.target === e.currentTarget) {
        emit('close');
    }
    backdropMouseDown.value = false;
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-60 flex items-center justify-center bg-black/80 backdrop-blur-sm"
            @mousedown="onBackdropMouseDown"
            @mouseup="onBackdropMouseUp"
        >
            <div
                class="border-border bg-panel flex h-[80vh] w-full max-w-3xl flex-col rounded-xl
                    border shadow-2xl"
            >
                <!-- Header -->
                <div
                    class="border-border flex shrink-0 items-center justify-between border-b px-4
                        py-3"
                >
                    <div class="min-w-0 flex-1">
                        <h3 class="text-sm font-semibold text-white">Search PornDB</h3>
                        <p class="text-dim mt-0.5 truncate text-xs">{{ scene.title }}</p>
                    </div>
                    <button
                        class="text-dim ml-4 shrink-0 transition-colors hover:text-white"
                        @click="emit('close')"
                    >
                        <Icon name="heroicons:x-mark" size="20" />
                    </button>
                </div>

                <!-- Content -->
                <div class="min-h-0 flex-1 overflow-hidden p-4">
                    <WatchSceneSearch :scene="sceneForSearch" @select="onSceneSelected" />
                </div>
            </div>
        </div>
    </Teleport>
</template>
