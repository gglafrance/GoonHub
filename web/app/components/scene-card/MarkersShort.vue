<script setup lang="ts">
import type { SceneCardMarker } from '~/types/marker';

// Markers icon shown next to the scene card tags; hovering lists the scene markers.
const props = defineProps<{
    sceneId: number;
    markers: SceneCardMarker[];
    badge?: boolean;
}>();

const { formatMarkerRange } = useFormatter();

// In the card template preview, links must not navigate away from the editor
const { blockInPreview } = useSceneCardPreview();

const showPopover = ref(false);
const trigger = ref<HTMLElement | null>(null);
const popoverStyle = ref<Record<string, string>>({});
let hideTimeout: ReturnType<typeof setTimeout> | null = null;

function updatePosition() {
    if (!trigger.value) return;
    const rect = trigger.value.getBoundingClientRect();
    popoverStyle.value = {
        top: `${rect.bottom + 4}px`,
        left: `${rect.left}px`,
    };
}

function onTriggerEnter() {
    if (hideTimeout) {
        clearTimeout(hideTimeout);
        hideTimeout = null;
    }
    updatePosition();
    showPopover.value = true;
}

function onTriggerLeave() {
    hideTimeout = setTimeout(() => {
        showPopover.value = false;
    }, 100);
}

function onPopoverEnter() {
    if (hideTimeout) {
        clearTimeout(hideTimeout);
        hideTimeout = null;
    }
}

function onPopoverLeave() {
    showPopover.value = false;
}
</script>

<template>
    <div
        ref="trigger"
        class="relative inline-flex items-center"
        @mouseenter="onTriggerEnter"
        @mouseleave="onTriggerLeave"
    >
        <button
            class="flex items-center gap-0.5 transition-colors"
            :class="
                badge
                    ? `bg-void/90 hover:bg-void rounded px-1.5 py-0.5 font-mono text-[10px]
                        font-medium text-white backdrop-blur-sm`
                    : 'text-dim hover:text-lava text-[10px]'
            "
            @click.stop.prevent
        >
            <Icon name="heroicons:bookmark" :size="badge ? '14' : '11'" />
            <span>{{ props.markers.length }}</span>
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
                    class="bg-surface border-border fixed z-50 max-h-48 min-w-36 overflow-y-auto
                        rounded-lg border p-1.5 shadow-lg backdrop-blur-md"
                    :style="popoverStyle"
                    @mouseenter="onPopoverEnter"
                    @mouseleave="onPopoverLeave"
                    @click.capture="blockInPreview"
                >
                    <NuxtLink
                        v-for="marker in props.markers"
                        :key="marker.id"
                        :to="markerWatchUrl({ ...marker, scene_id: sceneId })"
                        class="text-dim hover:text-lava flex items-center gap-1.5 rounded px-1.5
                            py-0.5 text-[10px] transition-colors hover:bg-white/5"
                        @click.stop
                    >
                        <span
                            class="h-1.5 w-1.5 shrink-0 rounded-full"
                            :style="{ backgroundColor: marker.color || '#FF4D4D' }"
                        />
                        <span class="min-w-0 flex-1 truncate">
                            {{ marker.label || 'No label' }}
                        </span>
                        <span class="shrink-0 font-mono opacity-60">
                            {{ formatMarkerRange(marker.timestamp, marker.end_timestamp) }}
                        </span>
                    </NuxtLink>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
