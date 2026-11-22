<script setup lang="ts">
import type { ContentWidth, GridDensity } from '~/types/settings';

// Quick display settings of the scene lists: card size, page width and infinite scroll (saved
// right away, also in Settings > App > Display)
defineProps<{
    tall?: boolean;
}>();

const settingsStore = useSettingsStore();

const open = ref(false);
const root = ref<HTMLElement | null>(null);

const densities: { value: GridDensity; label: string }[] = [
    { value: 'compact', label: 'Compact' },
    { value: 'comfortable', label: 'Comfortable' },
    { value: 'large', label: 'Large' },
];

const infiniteScroll = computed({
    get: () => settingsStore.infiniteScroll,
    set: (v: boolean) => settingsStore.setInfiniteScroll(v),
});

const setDensity = (density: GridDensity) => {
    if (density !== settingsStore.gridDensity) settingsStore.setGridDensity(density);
};

// A wider page fits more columns at every card size
const setWidth = (width: ContentWidth) => {
    if (width !== settingsStore.contentWidth) settingsStore.setContentWidth(width);
};

const onClickOutside = (event: MouseEvent) => {
    if (open.value && root.value && !root.value.contains(event.target as Node)) {
        open.value = false;
    }
};

onMounted(() => document.addEventListener('mousedown', onClickOutside));
onUnmounted(() => document.removeEventListener('mousedown', onClickOutside));
</script>

<template>
    <div ref="root" class="relative shrink-0" :class="tall ? 'h-10' : ''">
        <button
            type="button"
            class="border-border bg-panel text-dim flex h-full items-center rounded-lg border px-2
                py-1.5 transition-all hover:text-white"
            :class="{ 'border-lava/30 text-lava': open }"
            title="Display options"
            aria-label="Display options"
            :aria-expanded="open"
            @click="open = !open"
        >
            <Icon name="heroicons:adjustments-horizontal" size="14" />
        </button>

        <div
            v-if="open"
            class="border-border bg-panel absolute top-full right-0 z-30 mt-1 w-56 rounded-lg border
                p-2.5 shadow-xl"
        >
            <p class="text-dim mb-1.5 text-[10px] font-medium tracking-wider uppercase">
                Card size
            </p>
            <div class="border-border bg-void/50 flex rounded-lg border p-0.5">
                <button
                    v-for="opt in densities"
                    :key="opt.value"
                    type="button"
                    class="flex-1 rounded-md px-1.5 py-1 text-[11px] font-medium transition-all"
                    :class="
                        settingsStore.gridDensity === opt.value
                            ? 'bg-lava/15 text-lava'
                            : 'text-dim hover:text-white'
                    "
                    :aria-pressed="settingsStore.gridDensity === opt.value"
                    @click="setDensity(opt.value)"
                >
                    {{ opt.label }}
                </button>
            </div>

            <p class="text-dim mt-2.5 mb-1.5 text-[10px] font-medium tracking-wider uppercase">
                Page width
            </p>
            <div class="border-border bg-void/50 flex rounded-lg border p-0.5">
                <button
                    v-for="opt in CONTENT_WIDTH_OPTIONS"
                    :key="opt.value"
                    type="button"
                    class="flex-1 rounded-md px-1.5 py-1 text-[11px] font-medium transition-all"
                    :class="
                        settingsStore.contentWidth === opt.value
                            ? 'bg-lava/15 text-lava'
                            : 'text-dim hover:text-white'
                    "
                    :title="opt.hint"
                    :aria-pressed="settingsStore.contentWidth === opt.value"
                    @click="setWidth(opt.value)"
                >
                    {{ opt.label }}
                </button>
            </div>

            <div
                class="border-border mt-2.5 flex items-center justify-between gap-2 border-t pt-2.5"
            >
                <div>
                    <p class="text-[11px] font-medium text-white">Infinite scroll</p>
                    <p class="text-dim text-[10px]">Load more scenes on scroll</p>
                </div>
                <UiToggle v-model="infiniteScroll" />
            </div>
        </div>
    </div>
</template>
