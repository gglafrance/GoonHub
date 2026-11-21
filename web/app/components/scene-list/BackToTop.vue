<script setup lang="ts">
// Floating "back to top" button, shown once the page is scrolled down a few screens
const visible = ref(false);

const onScroll = () => {
    visible.value = window.scrollY > window.innerHeight * 1.5;
};

const toTop = () => window.scrollTo({ top: 0, behavior: 'smooth' });

onMounted(() => {
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
});
onUnmounted(() => window.removeEventListener('scroll', onScroll));
</script>

<template>
    <Teleport to="body">
        <Transition
            enter-active-class="transition duration-200"
            enter-from-class="translate-y-2 opacity-0"
            leave-active-class="transition duration-150"
            leave-to-class="translate-y-2 opacity-0"
        >
            <button
                v-if="visible"
                type="button"
                class="border-border bg-panel/80 hover:border-lava/40 hover:text-lava fixed right-4
                    bottom-20 z-30 flex h-10 w-10 items-center justify-center rounded-full border
                    text-white/80 shadow-lg backdrop-blur-md transition-colors sm:right-6"
                title="Back to top"
                aria-label="Back to top"
                @click="toTop"
            >
                <Icon name="heroicons:arrow-up" size="16" />
            </button>
        </Transition>
    </Teleport>
</template>
