<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// shorts widget: a horizontal "zapping" strip of short clips. Only the clips visible in the
// strip play (muted, hover to unmute); previous/next scroll, shuffle reloads the section.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ max_duration: number }>;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const strip = ref<HTMLElement | null>(null);
const cardEls = ref<HTMLElement[]>([]);
const visible = ref<Set<number>>(new Set());
let observer: IntersectionObserver | null = null;

const observe = () => {
    observer?.disconnect();
    if (!strip.value) return;
    observer = new IntersectionObserver(
        (entries) => {
            const next = new Set(visible.value);
            for (const entry of entries) {
                const id = Number((entry.target as HTMLElement).dataset.sceneId);
                if (entry.isIntersecting && entry.intersectionRatio >= 0.6) next.add(id);
                else next.delete(id);
            }
            visible.value = next;
        },
        { root: strip.value, threshold: [0, 0.6, 1] },
    );
    cardEls.value.forEach((el) => observer!.observe(el));
};

const setCardEl = (el: Element | null, index: number) => {
    if (el) cardEls.value[index] = el as HTMLElement;
};

const scrollBy = (direction: number) => {
    const el = strip.value;
    if (!el) return;
    el.scrollBy({ left: direction * el.clientWidth * 0.8, behavior: 'smooth' });
};

watch(
    () => props.data.scenes,
    async () => {
        cardEls.value = [];
        visible.value = new Set();
        await nextTick();
        strip.value?.scrollTo({ left: 0 });
        observe();
    },
);

onMounted(observe);
onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
    <div>
        <div
            v-if="data.scenes.length === 0"
            class="border-border text-dim flex h-40 items-center justify-center rounded-lg border
                border-dashed text-xs"
        >
            No clips shorter than {{ data.widget?.max_duration ?? 60 }}s
        </div>
        <div v-else class="relative">
            <div
                ref="strip"
                class="flex snap-x snap-mandatory gap-3 overflow-x-auto scroll-smooth pb-2"
                style="scrollbar-width: thin"
            >
                <div
                    v-for="(scene, index) in data.scenes"
                    :key="scene.id"
                    :ref="(el) => setCardEl(el as Element | null, index)"
                    :data-scene-id="scene.id"
                >
                    <HomepageWidgetsShortsShortCard
                        :scene="scene"
                        :active="visible.has(scene.id)"
                    />
                </div>
            </div>
            <div class="mt-2 flex items-center justify-between">
                <span class="text-dim font-mono text-[10px]">
                    {{ data.scenes.length }} clips · max {{ data.widget?.max_duration ?? 60 }}s
                </span>
                <div class="flex gap-1.5">
                    <button
                        class="border-border text-dim flex h-7 w-7 items-center justify-center
                            rounded-md border transition-colors hover:text-white"
                        title="Previous"
                        @click="scrollBy(-1)"
                    >
                        <Icon name="heroicons:chevron-left" size="14" />
                    </button>
                    <button
                        class="border-border text-dim flex h-7 w-7 items-center justify-center
                            rounded-md border transition-colors hover:text-white"
                        title="Next"
                        @click="scrollBy(1)"
                    >
                        <Icon name="heroicons:chevron-right" size="14" />
                    </button>
                    <button
                        class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                            items-center gap-1 rounded-md border px-2.5 text-[11px]
                            transition-colors"
                        @click="emit('refresh')"
                    >
                        <Icon name="heroicons:arrow-path" size="12" />
                        Shuffle
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
