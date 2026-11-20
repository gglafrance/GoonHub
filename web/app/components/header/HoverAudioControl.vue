<script setup lang="ts">
// Header control of the scene card preview sound: click toggles it, hovering shows a volume
// slider. The popover is teleported to the body so that the header bars cannot cover it.
const { enabled, volume, toggle, setVolume } = useHoverAudio();

const POPOVER_WIDTH = 192;

const trigger = ref<HTMLElement | null>(null);
const open = ref(false);
const popoverStyle = ref<Record<string, string>>({});
let hideTimeout: ReturnType<typeof setTimeout> | null = null;

const updatePosition = () => {
    if (!trigger.value) return;
    const rect = trigger.value.getBoundingClientRect();
    const left = Math.min(
        window.innerWidth - POPOVER_WIDTH - 8,
        Math.max(8, rect.right - POPOVER_WIDTH),
    );
    popoverStyle.value = {
        top: `${rect.bottom + 6}px`,
        left: `${left}px`,
        width: `${POPOVER_WIDTH}px`,
    };
};

const show = () => {
    if (hideTimeout) {
        clearTimeout(hideTimeout);
        hideTimeout = null;
    }
    updatePosition();
    open.value = true;
};

const hide = () => {
    hideTimeout = setTimeout(() => {
        open.value = false;
    }, 200);
};

const onInput = (event: Event) => {
    setVolume(Number((event.target as HTMLInputElement).value));
};

const onWheel = (event: WheelEvent) => {
    setVolume(volume.value + (event.deltaY < 0 ? 5 : -5));
};

const icon = computed(() => {
    if (!enabled.value || volume.value === 0) return 'heroicons:speaker-x-mark';
    return 'heroicons:speaker-wave';
});

onBeforeUnmount(() => {
    if (hideTimeout) clearTimeout(hideTimeout);
});
</script>

<template>
    <div ref="trigger" class="relative" @mouseenter="show" @mouseleave="hide">
        <button
            class="border-border flex h-7 w-7 items-center justify-center rounded-md border
                transition-all"
            :class="
                enabled && volume > 0
                    ? 'border-lava/30 text-lava'
                    : 'text-dim hover:border-lava/30 hover:text-lava'
            "
            :title="enabled ? 'Mute preview sound on hover' : 'Play preview sound on hover'"
            @click="toggle"
            @wheel.prevent="onWheel"
        >
            <Icon :name="icon" size="16" />
        </button>

        <Teleport to="body">
            <Transition
                enter-active-class="transition-opacity duration-100"
                enter-from-class="opacity-0"
                leave-active-class="transition-opacity duration-75"
                leave-to-class="opacity-0"
            >
                <div
                    v-if="open"
                    class="border-border bg-panel/95 fixed z-100 rounded-lg border p-3 shadow-lg
                        backdrop-blur-md"
                    :style="popoverStyle"
                    @mouseenter="show"
                    @mouseleave="hide"
                >
                    <div class="mb-2 flex items-center justify-between text-[11px]">
                        <span class="text-dim">Preview sound</span>
                        <span class="font-mono text-white">
                            {{ enabled ? `${volume}%` : 'Muted' }}
                        </span>
                    </div>
                    <input
                        type="range"
                        min="0"
                        max="100"
                        step="5"
                        :value="volume"
                        class="accent-lava w-full"
                        :class="{ 'opacity-50': !enabled }"
                        aria-label="Preview sound volume"
                        @input="onInput"
                    />
                    <p class="text-dim mt-1.5 text-[10px]">
                        Click the icon to mute, scroll on it to adjust.
                    </p>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
