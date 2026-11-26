<script setup lang="ts">
import type { MarkerWithScene } from '~/types/marker';

// A marker of the marker lists: thumbnail (static or animated, versioned URL), range, label and
// tags, scene title and age. With `to` it is a link (markers page); without, a button emitting
// `select` (search picker), with the selected and "already picked" states.
const props = defineProps<{
    marker: MarkerWithScene;
    to?: string;
    /** Animated thumbnails (processing setting), played while visible through `observe` */
    animated?: boolean;
    observe?: (el: HTMLVideoElement | null) => void;
    /** Picker: shows the selection checkbox */
    selectable?: boolean;
    selected?: boolean;
    /** Picker: badge of an item already picked (e.g. "Added") */
    marked?: string;
    disabled?: boolean;
}>();

const emit = defineEmits<{ select: [event: MouseEvent] }>();

defineSlots<{ actions?: () => unknown }>();

const { formatMarkerRange } = useFormatter();
const NuxtLinkComponent = resolveComponent('NuxtLink');

const showAnimated = computed(() => props.animated && !!props.marker.animated_thumbnail_path);

const onClick = (event: MouseEvent) => {
    if (!props.to) emit('select', event);
};
</script>

<template>
    <component
        :is="to ? NuxtLinkComponent : 'button'"
        :to="to"
        :type="to ? undefined : 'button'"
        :disabled="to ? undefined : disabled"
        :aria-pressed="to ? undefined : !!selected"
        class="group relative block w-full overflow-hidden rounded-lg text-left transition-transform
            duration-200 disabled:cursor-not-allowed"
        :class="[
            selected ? 'ring-lava/60 ring-2' : '',
            selectable ? 'border-border hover:border-border-hover border' : '',
        ]"
        @click="onClick"
    >
        <!-- Thumbnail -->
        <div class="relative aspect-video w-full overflow-hidden bg-black/40">
            <video
                v-if="showAnimated"
                :ref="(el) => observe?.(el as HTMLVideoElement)"
                :src="markerThumbnailUrl(marker.id, marker.updated_at, true)"
                muted
                loop
                playsinline
                preload="none"
                class="h-full w-full object-contain transition-transform duration-300
                    group-hover:scale-105"
            />
            <img
                v-else
                :src="markerThumbnailUrl(marker.id, marker.updated_at)"
                :alt="marker.scene_title"
                class="h-full w-full object-cover transition-transform duration-300
                    group-hover:scale-105"
                loading="lazy"
            />

            <!-- Gradient overlay -->
            <div
                class="pointer-events-none absolute inset-0 bg-linear-to-t from-black/80
                    via-black/20 to-transparent"
            />

            <!-- Timestamp badge -->
            <div
                class="absolute right-1.5 bottom-1.5 rounded bg-black/80 px-1.5 py-0.5 text-[10px]
                    font-semibold text-white/90 tabular-nums backdrop-blur-sm"
            >
                {{ formatMarkerRange(marker.timestamp, marker.end_timestamp) }}
            </div>

            <!-- Actions (e.g. add to a marker playlist), shown on hover -->
            <div
                v-if="$slots.actions"
                class="absolute top-1.5 right-1.5 z-10 opacity-0 transition-opacity
                    group-hover:opacity-100 focus-within:opacity-100"
            >
                <slot name="actions" />
            </div>

            <!-- Selection checkbox (picker) -->
            <div
                v-if="selectable"
                class="absolute top-1.5 right-1.5 z-10 flex h-5 w-5 items-center justify-center
                    rounded border transition-all"
                :class="
                    selected
                        ? 'bg-lava border-lava text-white'
                        : `bg-void/60 border-white/30 text-transparent backdrop-blur-sm
                            group-hover:text-white/50`
                "
            >
                <Icon name="heroicons:check" size="12" />
            </div>

            <!-- Label badge -->
            <div class="absolute top-1.5 left-1.5 flex max-w-[80%] items-center gap-1">
                <span
                    v-if="marker.label"
                    class="truncate rounded bg-black/70 px-1.5 py-0.5 text-[10px] font-medium
                        text-white/90 backdrop-blur-sm"
                >
                    {{ marker.label }}
                </span>
                <MarkersTagsBadge inline :tags="marker.tags" :label="marker.label" />
            </div>

            <!-- Already picked -->
            <div
                v-if="marked"
                class="bg-emerald/20 text-emerald absolute bottom-1.5 left-1.5 flex items-center
                    gap-1 rounded px-1.5 py-0.5 text-[10px] font-semibold backdrop-blur-sm"
            >
                <Icon name="heroicons:check-circle" size="11" />
                {{ marked }}
            </div>

            <div v-if="selected" class="bg-lava/10 pointer-events-none absolute inset-0" />
        </div>

        <!-- Info -->
        <div class="border-border bg-surface border-t px-2 py-1.5">
            <p class="truncate text-xs font-medium text-white group-hover:text-white">
                {{ marker.scene_title }}
            </p>
            <NuxtTime :datetime="marker.created_at" class="text-dim mt-0.5 text-[10px]" relative />
        </div>
    </component>
</template>
