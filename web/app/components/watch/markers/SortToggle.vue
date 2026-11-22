<script setup lang="ts">
// Order of the watch page markers list: by start time, or grouped by label.
import type { MarkerListSort } from '~/utils/markerList';

const sort = defineModel<MarkerListSort>({ required: true });
</script>

<template>
    <div
        class="border-border flex gap-0.5 rounded-md border p-0.5"
        role="group"
        aria-label="Sort markers"
    >
        <button
            v-for="option in MARKER_LIST_SORT_OPTIONS"
            :key="option.value"
            type="button"
            class="rounded px-2 py-0.5 text-[11px] font-medium transition-all"
            :class="
                sort === option.value
                    ? 'bg-white/10 text-white'
                    : 'text-dim hover:bg-white/5 hover:text-white'
            "
            :aria-pressed="sort === option.value"
            @click="sort = option.value"
        >
            {{ option.label }}
        </button>
    </div>
</template>
