<script setup lang="ts">
import type { QuickMarkerSuggestionRow } from '~/utils/quickMarker';

// Autocomplete rows of the quick marker inputs. Rendered inside the dialog (not teleported to
// the body) so that it also shows when the player is fullscreen.
defineProps<{
    rows: QuickMarkerSuggestionRow[];
    highlighted: number;
}>();

const emit = defineEmits<{
    pick: [index: number];
    hover: [index: number];
}>();
</script>

<template>
    <ul v-if="rows.length" class="qm-suggestions" role="listbox">
        <li
            v-for="(row, index) in rows"
            :key="row.key"
            role="option"
            :aria-selected="index === highlighted"
            class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-[11px]
                transition-colors"
            :class="
                index === highlighted
                    ? 'bg-lava/15 text-white'
                    : 'text-muted hover:bg-white/5 hover:text-white'
            "
            @mousedown.prevent="emit('pick', index)"
            @mousemove="emit('hover', index)"
        >
            <Icon v-if="row.create" name="heroicons:plus" size="11" class="text-lava shrink-0" />
            <span
                v-else-if="row.color"
                class="inline-block h-1.5 w-1.5 shrink-0 rounded-full"
                :style="{ backgroundColor: row.color }"
            />
            <span class="min-w-0 flex-1 truncate">{{ row.text }}</span>
            <span v-if="row.hint" class="text-dim shrink-0 font-mono text-[10px]">{{
                row.hint
            }}</span>
        </li>
    </ul>
</template>

<style scoped>
.qm-suggestions {
    margin-top: 6px;
    max-height: min(176px, 30vh);
    overflow-y: auto;
    padding: 2px;
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 8px;
    background: rgba(5, 5, 5, 0.6);
}
</style>
