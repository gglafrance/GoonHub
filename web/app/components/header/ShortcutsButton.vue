<script setup lang="ts">
// Opens the keyboard shortcuts modal (hidden on mobile).
const { shortcutsVisible } = useCommandPalette();
const showShortcuts = () => {
    shortcutsVisible.value = true;
};
</script>

<template>
    <button
        class="border-border text-dim hover:border-lava/30 hover:text-lava hidden h-7 w-7
            items-center justify-center rounded-md border transition-all sm:flex"
        title="Keyboard shortcuts"
        @click="showShortcuts"
    >
        <Icon name="heroicons:command-line" size="16" />
    </button>
</template>
