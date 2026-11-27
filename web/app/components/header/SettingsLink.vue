<script setup lang="ts">
// Link to the settings (required header item).
</script>

<template>
    <NuxtLink
        to="/settings"
        class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-7 w-7 items-center
            justify-center rounded-md border transition-all"
        title="Settings"
        aria-label="Settings"
    >
        <Icon name="heroicons:cog-6-tooth" size="16" />
    </NuxtLink>
</template>
