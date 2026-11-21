<script setup lang="ts">
// End of an infinite scroll list: loads the next page when it comes near the viewport, with a
// button fallback, and shows the back to top control.
const props = defineProps<{
    hasMore: boolean;
    loading: boolean;
    /** Total number of scenes, for the end message */
    total: number;
}>();

const emit = defineEmits<{
    loadMore: [];
}>();

const LOAD_MARGIN = 800;
const sentinel = ref<HTMLElement | null>(null);
const nearEnd = ref(false);
let observer: IntersectionObserver | null = null;

const maybeLoad = () => {
    if (nearEnd.value && props.hasMore && !props.loading) emit('loadMore');
};

onMounted(() => {
    observer = new IntersectionObserver(
        (entries) => {
            nearEnd.value = entries.some((e) => e.isIntersecting);
            maybeLoad();
        },
        { rootMargin: `0px 0px ${LOAD_MARGIN}px 0px` },
    );
    if (sentinel.value) observer.observe(sentinel.value);
});

onUnmounted(() => observer?.disconnect());

// A short page can leave the end in view after a load: keep going (measured, the observer only
// reports changes)
watch(
    () => props.loading,
    (loading) => {
        if (loading) return;
        nextTick(() => {
            const el = sentinel.value;
            if (!el) return;
            nearEnd.value = el.getBoundingClientRect().top < window.innerHeight + LOAD_MARGIN;
            maybeLoad();
        });
    },
);
</script>

<template>
    <div ref="sentinel" class="flex min-h-16 items-center justify-center py-6">
        <LoadingSpinner v-if="loading" label="Loading more scenes..." />
        <button
            v-else-if="hasMore"
            type="button"
            class="border-border bg-surface text-dim hover:border-border-hover rounded-lg border
                px-4 py-2 text-xs font-medium transition-all hover:text-white"
            @click="emit('loadMore')"
        >
            Load more
        </button>
        <p v-else-if="total > 0" class="text-dim text-[11px]">
            End of the list ({{ total }} scene{{ total !== 1 ? 's' : '' }})
        </p>
    </div>
    <SceneListBackToTop />
</template>
