<script setup lang="ts">
import type { VttCue } from '~/composables/useVttParser';

// Stash-like scrubber: hovering the bottom strip of a scene card shows the sprite frame
// under the cursor, and clicking opens the scene at that time.
const props = defineProps<{
    sceneId: number;
    duration: number;
    version?: string;
}>();

const emit = defineEmits<{
    'update:active': [active: boolean];
}>();

const router = useRouter();
const { formatDuration } = useFormatter();

const rootEl = ref<HTMLElement | null>(null);
const active = ref(false);
const percent = ref(0);
const cues = shallowRef<VttCue[]>([]);
const rootSize = ref({ width: 0, height: 0 });

const totalDuration = computed(() => {
    if (props.duration > 0) return props.duration;
    const last = cues.value[cues.value.length - 1];
    return last ? last.end : 0;
});

const time = computed(() => percent.value * totalDuration.value);

const frameStyle = computed(() => {
    const cue = findSpriteCue(cues.value, time.value);
    if (!cue || cue.w <= 0 || cue.h <= 0 || rootSize.value.width <= 0) return null;
    const scale = Math.min(rootSize.value.width / cue.w, rootSize.value.height / cue.h);
    return {
        width: `${cue.w}px`,
        height: `${cue.h}px`,
        backgroundImage: `url("${cue.url}")`,
        backgroundPosition: `-${cue.x}px -${cue.y}px`,
        backgroundRepeat: 'no-repeat',
        transform: `translate(-50%, -50%) scale(${scale})`,
    };
});

// Keeps the time label inside the card
const labelLeft = computed(() => `${Math.min(88, Math.max(12, percent.value * 100))}%`);

const setActive = (value: boolean) => {
    if (active.value === value) return;
    active.value = value;
    emit('update:active', value);
};

const handleEnter = () => {
    const rect = rootEl.value?.getBoundingClientRect();
    if (rect) rootSize.value = { width: rect.width, height: rect.height };
    setActive(true);
    loadSceneSpriteCues(props.sceneId, props.version).then((loaded) => {
        cues.value = loaded;
    });
};

const handleMove = (event: MouseEvent) => {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    if (rect.width <= 0) return;
    percent.value = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
};

const handleClick = () => {
    router.push(`/watch/${props.sceneId}?t=${Math.floor(time.value)}`);
};

onBeforeUnmount(() => setActive(false));
</script>

<template>
    <div ref="rootEl" class="pointer-events-none absolute inset-0 z-25">
        <!-- Sprite frame under the cursor -->
        <div v-if="active && frameStyle" class="bg-void absolute inset-0 overflow-hidden">
            <div class="absolute top-1/2 left-1/2" :style="frameStyle" />
        </div>

        <!-- Hover zone (bottom strip) -->
        <div
            class="pointer-events-auto absolute inset-x-0 bottom-0 h-1/4 cursor-pointer"
            @mouseenter="handleEnter"
            @mousemove="handleMove"
            @mouseleave="setActive(false)"
            @click.prevent.stop="handleClick"
        >
            <template v-if="active">
                <div
                    class="absolute bottom-2 -translate-x-1/2 rounded bg-black/70 px-1.5 py-0.5
                        font-mono text-[10px] text-white backdrop-blur-sm"
                    :style="{ left: labelLeft }"
                >
                    {{ formatDuration(time) }}
                </div>
                <div class="absolute inset-x-0 bottom-0 h-1 bg-white/20">
                    <div class="bg-lava h-full" :style="{ width: `${percent * 100}%` }" />
                </div>
            </template>
            <!-- Hint while the scrubber is idle -->
            <div
                v-else
                class="absolute inset-x-0 bottom-0 h-0.5 bg-white/10 opacity-0 transition-opacity
                    group-hover:opacity-100"
            />
        </div>
    </div>
</template>
