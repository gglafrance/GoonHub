<script setup lang="ts">
import type { MarkerTagInfo } from '~/types/marker';

// "+X" badge over a marker cover: the marker tags other than its label, listed on hover.
const props = defineProps<{
    tags?: Pick<MarkerTagInfo, 'id' | 'name' | 'color'>[] | null;
    label: string;
    // Inline (next to another badge) instead of positioned on the cover's top left corner
    inline?: boolean;
}>();

const extraTags = computed(() => {
    const label = props.label.trim().toLowerCase();
    return (props.tags ?? []).filter((t) => t.name.trim().toLowerCase() !== label);
});

const show = ref(false);
const trigger = ref<HTMLElement | null>(null);
const popoverStyle = ref<Record<string, string>>({});
let hideTimeout: ReturnType<typeof setTimeout> | null = null;

const onEnter = () => {
    if (hideTimeout) {
        clearTimeout(hideTimeout);
        hideTimeout = null;
    }
    if (trigger.value) {
        const rect = trigger.value.getBoundingClientRect();
        popoverStyle.value = { top: `${rect.bottom + 4}px`, left: `${rect.left}px` };
    }
    show.value = true;
};

const onLeave = () => {
    hideTimeout = setTimeout(() => (show.value = false), 100);
};

onBeforeUnmount(() => {
    if (hideTimeout) clearTimeout(hideTimeout);
});
</script>

<template>
    <span
        v-if="extraTags.length"
        ref="trigger"
        class="bg-void/80 z-10 shrink-0 cursor-default rounded px-1.5 py-0.5 font-mono text-[10px]
            font-medium text-white backdrop-blur-sm"
        :class="inline ? 'relative' : 'absolute top-1.5 left-1.5'"
        @mouseenter="onEnter"
        @mouseleave="onLeave"
        @click.stop.prevent
    >
        +{{ extraTags.length }}
        <Teleport to="body">
            <Transition
                enter-active-class="transition-opacity duration-100"
                enter-from-class="opacity-0"
                leave-active-class="transition-opacity duration-75"
                leave-to-class="opacity-0"
            >
                <span
                    v-if="show"
                    class="bg-surface border-border fixed z-50 flex max-w-56 flex-col gap-0.5
                        rounded-lg border p-1.5 shadow-lg backdrop-blur-md"
                    :style="popoverStyle"
                    @mouseenter="onEnter"
                    @mouseleave="onLeave"
                >
                    <span
                        v-for="tag in extraTags"
                        :key="tag.id"
                        class="text-dim flex items-center gap-1.5 truncate px-1 text-[10px]"
                    >
                        <span
                            class="h-1.5 w-1.5 shrink-0 rounded-full"
                            :style="{ backgroundColor: tag.color || '#FFFFFF' }"
                        />
                        {{ tag.name }}
                    </span>
                </span>
            </Transition>
        </Teleport>
    </span>
</template>
