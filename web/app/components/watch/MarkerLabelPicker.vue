<script setup lang="ts">
import type { MarkerLabelSuggestion } from '~/types/marker';

const props = defineProps<{
    visible: boolean;
    suggestions: MarkerLabelSuggestion[];
    anchorEl: HTMLElement | null;
    selectedIndex: number;
}>();

const emit = defineEmits<{
    select: [suggestion: MarkerLabelSuggestion];
    close: [];
}>();

const dropdownStyle = ref<{ top: string; left: string; minWidth: string }>({
    top: '0px',
    left: '0px',
    minWidth: '200px',
});
const dropdownRef = ref<HTMLElement | null>(null);

function updatePosition() {
    if (!props.anchorEl) return;
    const rect = props.anchorEl.getBoundingClientRect();
    dropdownStyle.value = {
        top: `${rect.bottom + 4}px`,
        left: `${rect.left}px`,
        minWidth: `${rect.width}px`,
    };
}

function onClickOutside(event: MouseEvent) {
    const target = event.target as Node;
    if (
        dropdownRef.value &&
        !dropdownRef.value.contains(target) &&
        props.anchorEl &&
        !props.anchorEl.contains(target)
    ) {
        emit('close');
    }
}

watch(
    () => props.visible,
    (open) => {
        if (open) {
            updatePosition();
            setTimeout(() => document.addEventListener('click', onClickOutside), 0);
            window.addEventListener('scroll', updatePosition, true);
        } else {
            document.removeEventListener('click', onClickOutside);
            window.removeEventListener('scroll', updatePosition, true);
        }
    },
);

// Update position when suggestions change (in case content height changes)
watch(
    () => props.suggestions,
    () => {
        if (props.visible) {
            updatePosition();
        }
    },
);

onBeforeUnmount(() => {
    document.removeEventListener('click', onClickOutside);
    window.removeEventListener('scroll', updatePosition, true);
});
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible && suggestions.length > 0"
            ref="dropdownRef"
            class="border-border bg-panel fixed z-[9999] max-h-48 overflow-y-auto rounded-lg border
                shadow-xl"
            :style="dropdownStyle"
        >
            <button
                v-for="(suggestion, index) in suggestions"
                :key="suggestion.label"
                type="button"
                class="flex w-full items-center justify-between px-3 py-2 text-left text-xs
                    transition-colors"
                :class="[
                    index === selectedIndex
                        ? 'bg-surface-hover text-white'
                        : 'text-dim hover:bg-surface-hover hover:text-white',
                ]"
                @mousedown.prevent="emit('select', suggestion)"
            >
                <span class="truncate">{{ suggestion.label }}</span>
                <span class="text-dim/50 ml-2 shrink-0 text-[10px]">{{ suggestion.count }}x</span>
            </button>
        </div>
    </Teleport>
</template>
