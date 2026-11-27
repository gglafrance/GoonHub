<script setup lang="ts">
// Safe mode toggle (blurs the thumbnails and previews).
const { enabled: safeModeEnabled, toggle: toggleSafeMode } = useSafeMode();
</script>

<template>
    <button
        class="border-border flex h-7 w-7 items-center justify-center rounded-md border
            transition-all"
        :class="
            safeModeEnabled
                ? 'border-lava/30 text-lava'
                : 'text-dim hover:border-lava/30 hover:text-lava'
        "
        :title="safeModeEnabled ? 'Disable safe mode' : 'Enable safe mode'"
        @click="toggleSafeMode"
    >
        <Icon :name="safeModeEnabled ? 'heroicons:eye-slash' : 'heroicons:eye'" size="16" />
    </button>
</template>
