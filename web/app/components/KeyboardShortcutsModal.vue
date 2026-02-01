<script setup lang="ts">
const props = defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
}>();

const { layout, displayKeys } = useKeyboardLayout();

const shortcuts = computed(() => [
    {
        category: 'Playback',
        items: [
            { keys: ['Space', 'K'], description: 'Play / Pause' },
            { keys: ['F'], description: 'Toggle fullscreen' },
            { keys: ['T'], description: 'Toggle theater mode' },
            { keys: ['P'], description: 'Picture-in-Picture' },
        ],
    },
    {
        category: 'Seeking',
        items: [
            { keys: ['←'], description: 'Seek back 5s' },
            { keys: ['→'], description: 'Seek forward 5s' },
            { keys: ['J'], description: 'Seek back 10s' },
            { keys: ['L'], description: 'Seek forward 10s' },
            { keys: [displayKeys.value.frameBack], description: 'Previous frame (paused)' },
            { keys: [displayKeys.value.frameForward], description: 'Next frame (paused)' },
        ],
    },
    {
        category: 'Audio',
        items: [
            { keys: ['↑', '+'], description: 'Volume up' },
            { keys: ['↓', '-'], description: 'Volume down' },
            { keys: ['N'], description: 'Toggle mute' },
        ],
    },
    {
        category: 'Speed',
        items: [
            { keys: [displayKeys.value.speedDecrease], description: 'Decrease speed' },
            { keys: [displayKeys.value.speedIncrease], description: 'Increase speed' },
        ],
    },
    {
        category: 'Markers',
        items: [{ keys: ['M'], description: 'Add marker at current time' }],
    },
    {
        category: 'Navigation',
        items: [
            { keys: [displayKeys.value.pagePrev], description: 'Previous page' },
            { keys: [displayKeys.value.pageNext], description: 'Next page' },
        ],
    },
]);

const handleKeydown = (e: KeyboardEvent) => {
    if (e.key === 'Escape' && props.visible) {
        emit('close');
    }
};

onMounted(() => {
    window.addEventListener('keydown', handleKeydown);
});

onUnmounted(() => {
    window.removeEventListener('keydown', handleKeydown);
});
</script>

<template>
    <Teleport to="body">
        <Transition
            enter-active-class="transition duration-200 ease-out"
            enter-from-class="opacity-0"
            enter-to-class="opacity-100"
            leave-active-class="transition duration-150 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
        >
            <div
                v-if="visible"
                class="fixed inset-0 z-100 flex items-center justify-center p-4"
                @click.self="emit('close')"
            >
                <!-- Backdrop -->
                <div class="bg-void/80 absolute inset-0 backdrop-blur-sm" />

                <!-- Modal -->
                <div
                    class="border-border bg-surface relative w-full max-w-lg overflow-hidden
                        rounded-xl border shadow-2xl"
                >
                    <!-- Header -->
                    <div class="border-border flex items-center justify-between border-b px-5 py-4">
                        <div class="flex items-center gap-3">
                            <div
                                class="bg-lava/10 flex h-8 w-8 items-center justify-center
                                    rounded-lg"
                            >
                                <Icon name="heroicons:command-line" size="16" class="text-lava" />
                            </div>
                            <div>
                                <h2 class="text-sm font-semibold text-white">Keyboard Shortcuts</h2>
                                <p class="text-dim text-[11px]">
                                    Video player controls
                                    <span class="text-lava ml-1 uppercase">{{ layout }}</span>
                                </p>
                            </div>
                        </div>
                        <button
                            class="text-dim hover:text-lava hover:bg-lava/10 -mr-1 flex items-center
                                justify-center rounded-lg p-1.5 transition-all"
                            @click="emit('close')"
                        >
                            <Icon name="heroicons:x-mark" size="18" />
                        </button>
                    </div>

                    <!-- Content -->
                    <div class="max-h-[60vh] overflow-y-auto p-5">
                        <div class="grid gap-5 sm:grid-cols-2">
                            <div
                                v-for="section in shortcuts"
                                :key="section.category"
                                class="space-y-2"
                            >
                                <h3
                                    class="text-lava text-[10px] font-semibold tracking-wider
                                        uppercase"
                                >
                                    {{ section.category }}
                                </h3>
                                <div class="space-y-1.5">
                                    <div
                                        v-for="(shortcut, idx) in section.items"
                                        :key="idx"
                                        class="flex items-center justify-between gap-3"
                                    >
                                        <span class="text-muted text-xs">{{
                                            shortcut.description
                                        }}</span>
                                        <div class="flex items-center gap-1">
                                            <template
                                                v-for="(key, keyIdx) in shortcut.keys"
                                                :key="keyIdx"
                                            >
                                                <span
                                                    v-if="keyIdx > 0"
                                                    class="text-dim text-[10px]"
                                                >
                                                    /
                                                </span>
                                                <kbd
                                                    class="border-border bg-panel inline-flex
                                                        min-w-6 items-center justify-center rounded
                                                        border px-1.5 py-0.5 font-mono text-[11px]
                                                        font-medium text-white"
                                                >
                                                    {{ key }}
                                                </kbd>
                                            </template>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Footer -->
                    <div class="border-border border-t px-5 py-3">
                        <p class="text-dim text-center text-[10px]">
                            Press
                            <kbd
                                class="border-border bg-panel mx-1 inline-flex items-center
                                    justify-center rounded border px-1.5 py-0.5 font-mono
                                    text-[10px] font-medium text-white"
                            >
                                Esc
                            </kbd>
                            to close
                        </p>
                    </div>
                </div>
            </div>
        </Transition>
    </Teleport>
</template>
