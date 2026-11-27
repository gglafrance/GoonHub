<script setup lang="ts">
// Command palette: compact search field on desktop, icon on mobile.
const { open: openPalette, modifierLabel } = useCommandPalette();
</script>

<template>
    <button
        class="border-border bg-panel/60 text-dim hover:border-lava/30 group flex h-7 w-7
            items-center justify-center gap-2 rounded-md border transition-all hover:text-white
            sm:w-56 sm:justify-start sm:px-2 lg:w-64"
        :title="`Search and commands (${modifierLabel}+K)`"
        aria-label="Open the command palette"
        @click="openPalette()"
    >
        <Icon
            name="heroicons:magnifying-glass"
            size="14"
            class="group-hover:text-lava shrink-0 transition-colors"
        />
        <span class="hidden flex-1 truncate text-left text-[11px] sm:inline">
            Search or jump to...
        </span>
        <kbd
            class="border-border bg-surface hidden shrink-0 items-center gap-0.5 rounded border px-1
                py-px font-mono text-[10px] sm:inline-flex"
        >
            {{ modifierLabel }} K
        </kbd>
    </button>
</template>
