<script setup lang="ts">
const props = defineProps<{
    tags: string[];
}>();

const showPopover = ref(false);
const trigger = ref<HTMLElement | null>(null);
const popoverStyle = ref<Record<string, string>>({});

function updatePosition() {
    if (!trigger.value) return;
    const rect = trigger.value.getBoundingClientRect();
    popoverStyle.value = {
        top: `${rect.bottom + 4}px`,
        left: `${rect.left}px`,
    };
}

function onEnter() {
    updatePosition();
    showPopover.value = true;
}
</script>

<template>
    <div
        ref="trigger"
        class="relative inline-flex items-center"
        @mouseenter="onEnter"
        @mouseleave="showPopover = false"
    >
        <button class="text-dim hover:text-lava flex items-center gap-0.5 text-[10px] transition-colors" @click.stop.prevent>
            <Icon name="heroicons:tag" size="11" />
            <span>{{ props.tags.length }}</span>
        </button>

        <Teleport to="body">
            <Transition
                enter-active-class="transition-opacity duration-100"
                enter-from-class="opacity-0"
                leave-active-class="transition-opacity duration-75"
                leave-to-class="opacity-0"
            >
                <div
                    v-if="showPopover"
                    class="bg-surface border-border fixed z-50 max-h-40 min-w-28 overflow-y-auto rounded-lg border p-1.5 shadow-lg backdrop-blur-md"
                    :style="popoverStyle"
                >
                    <NuxtLink
                        v-for="tag in props.tags"
                        :key="tag"
                        :to="{ path: '/search', query: { tags: tag } }"
                        target="_blank"
                        class="text-dim hover:text-lava hover:bg-white/5 block truncate rounded px-1.5 py-0.5 text-[10px] transition-colors"
                        @click.stop
                    >
                        {{ tag }}
                    </NuxtLink>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
