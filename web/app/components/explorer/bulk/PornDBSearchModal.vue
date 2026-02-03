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

// Convert SceneMatchInfo to minimal object for WatchSceneSearch
const sceneForSearch = computed(() => ({
    title: props.scene.title,
    studio: props.scene.studio,
}));

function onSceneSelected(scene: PornDBScene) {
    emit('select', scene);
    emit('close');
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-[60] flex items-center justify-center bg-black/80
                backdrop-blur-sm"
            @click.self="emit('close')"
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
                        @click="emit('close')"
                        class="text-dim ml-4 shrink-0 transition-colors hover:text-white"
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
