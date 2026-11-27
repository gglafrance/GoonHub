<script setup lang="ts">
import type { NavLayout } from '~/types/navLayout';

// Live preview of the draft layout with the real header bars (not interactive). The bars are
// laid out at a desktop width (PREVIEW_WIDTH, so the left, center and right zones sit where they
// will on screen) and scaled down to fit the settings column: every item stays visible.
const props = defineProps<{ layout: NavLayout }>();
const perms = useMyPermissions();

const resolved = computed(() => resolveNavLayout(props.layout, perms.has));
const showNavRow = computed(() => !isNavBarEmpty(resolved.value.nav));

const PREVIEW_WIDTH = 1440;
const frame = ref<HTMLElement | null>(null);
const content = ref<HTMLElement | null>(null);
const scale = ref(1);
const height = ref<number | null>(null);

const measure = () => {
    if (!frame.value || !content.value) return;
    const width = Math.max(PREVIEW_WIDTH, content.value.scrollWidth);
    scale.value = Math.min(1, frame.value.clientWidth / width);
    height.value = content.value.offsetHeight * scale.value;
};

let observer: ResizeObserver | null = null;
onMounted(() => {
    observer = new ResizeObserver(measure);
    if (frame.value) observer.observe(frame.value);
    if (content.value) observer.observe(content.value);
    measure();
});
onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-1 text-sm font-semibold text-white">Preview</h3>
        <p class="text-dim mb-3 text-[11px]">Your header with the changes below.</p>
        <div
            ref="frame"
            class="border-border overflow-hidden rounded-lg border bg-[#050505]"
            :style="height !== null ? { height: `${height}px` } : undefined"
        >
            <div
                ref="content"
                class="origin-top-left"
                :style="{ width: `${PREVIEW_WIDTH}px`, transform: `scale(${scale})` }"
                inert
            >
                <div class="border-border bg-void/80 border-b px-4">
                    <HeaderLayoutBar bar="top" :zones="resolved.top" />
                </div>
                <div v-if="showNavRow" class="bg-void/60 px-4">
                    <HeaderLayoutBar bar="nav" :zones="resolved.nav" />
                </div>
                <div v-else class="text-dim px-4 py-2 text-[11px]">
                    The nav row is empty: it is not shown.
                </div>
            </div>
        </div>
    </div>
</template>
